package services

import (
	"context"
	"fmt"

	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateDatabaseRole creates a database role by issuing CREATE ROLE DDL in
// the parent database. The database's existence is verified first so a
// missing database surfaces as its own error rather than a DDL failure.
func (s *SpannerService) CreateDatabaseRole(ctx context.Context, parent string, roleId string) (*databasepb.DatabaseRole, error) {
	// Validate arguments
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	// Ensure role is provided
	if roleId == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument roleId, field is required but not provided")
	}

	// Verify the database exists before issuing DDL
	if _, err := s.conn.Dialect(ctx, parent); err != nil {
		return nil, err
	}

	if err := s.conn.ExecuteDDL(ctx, parent, schema.CreateRoleDdl(roleId)); err != nil {
		return nil, err
	}

	return &databasepb.DatabaseRole{
		Name: fmt.Sprintf("%s/databaseRoles/%s", parent, roleId),
	}, nil
}

// GetDatabaseRole fetches a database role by its full resource name. The
// Admin API has no per-role lookup, so every role in the database is listed
// and matched by name; codes.NotFound is returned if the role is absent.
func (s *SpannerService) GetDatabaseRole(ctx context.Context, name string) (*databasepb.DatabaseRole, error) {
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlDatabaseRoleNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlDatabaseRoleNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	roleName, err := names.ParseDatabaseRole(name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s): %v", name, err)
	}
	database := roleName.DatabaseName().String()

	// List all roles (unpaged) and find the requested one.
	roleNames, _, err := s.conn.DatabaseRoles(ctx, database, 0, "")
	if err != nil {
		return nil, err
	}

	for _, roleName := range roleNames {
		if roleName == name {
			return &databasepb.DatabaseRole{Name: roleName}, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "Database role (%s) not found", name)
}

// ListDatabaseRoles lists the roles of the parent database one page at a
// time; the returned page token is empty on the final page.
func (s *SpannerService) ListDatabaseRoles(ctx context.Context, parent string, pageSize int32, pageToken string) ([]*databasepb.DatabaseRole, string, error) {
	// Validate parent
	googleSqlValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, "", status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	roleNames, nextPageToken, err := s.conn.DatabaseRoles(ctx, parent, pageSize, pageToken)
	if err != nil {
		return nil, "", err
	}

	res := make([]*databasepb.DatabaseRole, 0, len(roleNames))
	for _, roleName := range roleNames {
		res = append(res, &databasepb.DatabaseRole{Name: roleName})
	}

	return res, nextPageToken, nil
}

// DeleteDatabaseRole removes a database role by issuing DROP ROLE DDL in its
// database.
func (s *SpannerService) DeleteDatabaseRole(ctx context.Context, name string) error {
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlDatabaseRoleNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlDatabaseRoleNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	roleName, err := names.ParseDatabaseRole(name)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "Invalid argument name (%s): %v", name, err)
	}
	roleId := roleName.Role
	database := roleName.DatabaseName().String()

	// Verify the database exists before issuing DDL
	if _, err := s.conn.Dialect(ctx, database); err != nil {
		return err
	}

	return s.conn.ExecuteDDL(ctx, database, schema.DropRoleDdl(roleId))
}
