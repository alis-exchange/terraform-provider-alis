package services

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-alis/internal/spanner/conn"
	"terraform-provider-alis/internal/utils"
)

func (s *SpannerService) CreateDatabaseRole(ctx context.Context, parent string, roleId string) (*databasepb.DatabaseRole, error) {
	// Validate arguments
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Ensure role is provided
	if roleId == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument roleId, field is required but not provided")
	}

	// Get database dialect (also verifies the database exists)
	dialect, err := s.conn.Dialect(ctx, parent)
	if err != nil {
		return nil, err
	}

	// CREATE ROLE inventory_admin;
	var ddlStatements []string
	if dialect == conn.DialectGoogleSQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("CREATE ROLE %s", roleId))
	}
	if dialect == conn.DialectPostgreSQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("CREATE ROLE %s", roleId))
	}
	if err := s.conn.ExecuteDDL(ctx, parent, ddlStatements...); err != nil {
		return nil, err
	}

	return &databasepb.DatabaseRole{
		Name: fmt.Sprintf("%s/databaseRoles/%s", parent, roleId),
	}, nil
}

func (s *SpannerService) GetDatabaseRole(ctx context.Context, name string) (*databasepb.DatabaseRole, error) {
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlDatabaseRoleNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlDatabaseRoleNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	// Decompose name to get project, instance, database
	nameParts := strings.Split(name, "/")
	project := nameParts[1]
	instance := nameParts[3]
	databaseId := nameParts[5]
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId)

	// List all roles (unpaged) and find the requested one, preserving the
	// historical iterate-until-match behavior.
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

func (s *SpannerService) DeleteDatabaseRole(ctx context.Context, name string) error {
	// Validate name
	googleSqlValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlDatabaseRoleNameRegex)
	postgresSqlValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlDatabaseRoleNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	// Decompose name to get project, instance, database
	nameParts := strings.Split(name, "/")
	project := nameParts[1]
	instance := nameParts[3]
	databaseId := nameParts[5]
	roleId := nameParts[7]
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId)

	// Get database dialect (also verifies the database exists)
	dialect, err := s.conn.Dialect(ctx, database)
	if err != nil {
		return err
	}

	var ddlStatements []string
	if dialect == conn.DialectGoogleSQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("DROP ROLE %s", roleId))
	}
	if dialect == conn.DialectPostgreSQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("DROP ROLE %s", roleId))
	}
	return s.conn.ExecuteDDL(ctx, database, ddlStatements...)
}
