package services

import (
	"context"

	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SetTableIamBinding makes the role's privileges on the table named by parent
// match binding exactly: missing permissions are granted and permissions the
// role holds that binding omits are revoked, in one DDL batch. The binding is
// authoritative for its own role only — other roles on the table keep their
// grants.
func (s *SpannerService) SetTableIamBinding(ctx context.Context, parent string, binding *TablePolicyBinding) (*TablePolicyBinding, error) {
	// Validate arguments
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSqlTableNameRegex,
		utils.SpannerPostgresSqlTableNameRegex,
	); err != nil {
		return nil, err
	}

	// Ensure binding is provided
	if binding == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument binding, field is required but not provided")
	}

	// The role reaches GRANT/REVOKE by concatenation, so it must be a bare
	// identifier — never a fragment that could carry its own DDL.
	if err := utils.ValidateDialectArgument(
		"binding.role",
		binding.Role,
		utils.SpannerGoogleSqlRoleIdRegex,
		utils.SpannerPostgresSqlRoleIdRegex,
	); err != nil {
		return nil, err
	}

	// Ensure permissions are provided
	if len(binding.Permissions) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument binding.permissions, field is required but not provided")
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableId := parentName.Table

	// Verify the database exists before issuing DDL
	if _, err := s.conn.Dialect(ctx, database); err != nil {
		return nil, err
	}

	granted, err := s.grantedPermissions(ctx, parent, binding.Role)
	if err != nil {
		return nil, err
	}

	desired := make(map[TablePolicyBindingPermission]bool, len(binding.Permissions))
	for _, permission := range binding.Permissions {
		desired[permission] = true
	}

	var toGrant, toRevoke []TablePolicyBindingPermission
	for _, permission := range TablePolicyBindingPermissions {
		switch {
		case desired[permission] && !granted[permission]:
			toGrant = append(toGrant, permission)
		case !desired[permission] && granted[permission]:
			toRevoke = append(toRevoke, permission)
		}
	}

	var statements []string
	if len(toRevoke) > 0 {
		statements = append(statements, schema.RevokeTablePrivilegesDdl(tableId, binding.Role, permissionNames(toRevoke)))
	}
	if len(toGrant) > 0 {
		statements = append(statements, schema.GrantTablePrivilegesDdl(tableId, binding.Role, permissionNames(toGrant)))
	}

	if err := s.conn.ExecuteDDL(ctx, database, statements...); err != nil {
		return nil, err
	}

	return binding, nil
}

// grantedPermissions reports the permissions role currently holds on the table
// as a set. Holding none is not an error here: that is the ordinary starting
// state when the binding is first created.
func (s *SpannerService) grantedPermissions(ctx context.Context, parent, role string) (map[TablePolicyBindingPermission]bool, error) {
	granted := map[TablePolicyBindingPermission]bool{}

	existing, err := s.GetTableIamBinding(ctx, parent, role)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return granted, nil
		}

		return nil, err
	}

	for _, permission := range existing.Permissions {
		granted[permission] = true
	}

	return granted, nil
}

// GetTableIamBinding reads the permissions currently granted to role on the
// table from INFORMATION_SCHEMA.TABLE_PRIVILEGES. codes.NotFound is returned
// when the role holds no privileges on the table.
func (s *SpannerService) GetTableIamBinding(ctx context.Context, parent, role string) (*TablePolicyBinding, error) {
	// Validate arguments
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSqlTableNameRegex,
		utils.SpannerPostgresSqlTableNameRegex,
	); err != nil {
		return nil, err
	}

	if err := utils.ValidateDialectArgument(
		"role",
		role,
		utils.SpannerGoogleSqlRoleIdRegex,
		utils.SpannerPostgresSqlRoleIdRegex,
	); err != nil {
		return nil, err
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableId := parentName.Table

	var rows []*TablePermissionsRow
	if err := s.conn.Query(ctx, database, &rows,
		"SELECT * FROM INFORMATION_SCHEMA.TABLE_PRIVILEGES WHERE table_name = ? AND grantee = ?", tableId, role); err != nil {
		return nil, status.Errorf(codes.Internal, "Error getting table IAM binding: %v", err)
	}

	if len(rows) == 0 {
		return nil, status.Errorf(codes.NotFound, "Table IAM binding %s not found", role)
	}

	binding := &TablePolicyBinding{
		Role: role,
	}
	for _, row := range rows {
		if row.GetPermission() == TablePolicyBindingPermission_UNSPECIFIED {
			continue
		}

		binding.Permissions = append(binding.Permissions, row.GetPermission())
	}

	return binding, nil
}

// DeleteTableIamBinding revokes every permission the role currently holds on
// the table; the existing grants are read first so the REVOKE covers exactly
// what is present. Having nothing to revoke — no grants left, or no database
// to revoke them in — is success, so a destroy still converges after the
// grants were dropped out of band.
func (s *SpannerService) DeleteTableIamBinding(ctx context.Context, parent, role string) error {
	// Validate arguments
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSqlTableNameRegex,
		utils.SpannerPostgresSqlTableNameRegex,
	); err != nil {
		return err
	}

	if err := utils.ValidateDialectArgument(
		"role",
		role,
		utils.SpannerGoogleSqlRoleIdRegex,
		utils.SpannerPostgresSqlRoleIdRegex,
	); err != nil {
		return err
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableId := parentName.Table

	// Verify the database exists before issuing DDL
	if _, err := s.conn.Dialect(ctx, database); err != nil {
		if status.Code(err) == codes.NotFound {
			return nil
		}

		return err
	}

	granted, err := s.grantedPermissions(ctx, parent, role)
	if err != nil {
		return err
	}

	var permissions []TablePolicyBindingPermission
	for _, permission := range TablePolicyBindingPermissions {
		if granted[permission] {
			permissions = append(permissions, permission)
		}
	}
	// Nothing granted: a REVOKE with no privileges is not valid DDL.
	if len(permissions) == 0 {
		return nil
	}

	return s.conn.ExecuteDDL(ctx, database, schema.RevokeTablePrivilegesDdl(tableId, role, permissionNames(permissions)))
}
