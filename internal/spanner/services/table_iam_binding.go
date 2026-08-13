package services

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"
)

func (s *SpannerService) SetTableIamBinding(ctx context.Context, parent string, binding *TablePolicyBinding) (*TablePolicyBinding, error) {
	// Validate arguments
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Ensure binding is provided
	if binding == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument binding, field is required but not provided")
	}

	// Ensure role is provided
	if binding.Role == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument binding.role, field is required but not provided")
	}

	// Ensure permissions are provided
	if len(binding.Permissions) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument binding.permissions, field is required but not provided")
	}

	// Deconstruct parent name to get project, instance, database and table
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId)

	// Verify the database exists before issuing DDL
	if _, err := s.conn.Dialect(ctx, database); err != nil {
		return nil, err
	}

	var permissions []string
	for _, permission := range binding.Permissions {
		permissions = append(permissions, permission.String())
	}

	if err := s.conn.ExecuteDDL(ctx, database, schema.GrantTablePrivilegesDdl(tableId, binding.Role, permissions)); err != nil {
		return nil, err
	}

	return binding, nil
}

func (s *SpannerService) GetTableIamBinding(ctx context.Context, parent string, role string) (*TablePolicyBinding, error) {
	// Validate arguments
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Ensure role is provided
	if role == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument role, field is required but not provided")
	}

	// Deconstruct parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId)

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

func (s *SpannerService) DeleteTableIamBinding(ctx context.Context, parent string, role string) error {
	// Validate arguments
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Ensure role is provided
	if role == "" {
		return status.Error(codes.InvalidArgument, "Invalid argument role, field is required but not provided")
	}

	// Deconstruct parent name to get project, instance and database id
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId)

	// Verify the database exists before issuing DDL
	if _, err := s.conn.Dialect(ctx, database); err != nil {
		return err
	}

	// Get binding
	binding, err := s.GetTableIamBinding(ctx, parent, role)
	if err != nil {
		return err
	}

	var permissions []string
	for _, permission := range binding.Permissions {
		permissions = append(permissions, permission.String())
	}

	return s.conn.ExecuteDDL(ctx, database, schema.RevokeTablePrivilegesDdl(tableId, role, permissions))
}
