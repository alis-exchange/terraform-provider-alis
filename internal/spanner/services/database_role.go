package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"terraform-provider-alis/internal/utils"

	spanner "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Get database state
	database, err := client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: parent,
	})
	if err != nil {
		return nil, err
	}

	// CREATE ROLE inventory_admin;
	var ddlStatements []string
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("CREATE ROLE %s", roleId))
	}
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("CREATE ROLE %s", roleId))
	}
	updateDatabaseDdlOperation, err := client.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   database.GetName(),
		Statements: ddlStatements,
	})
	if err != nil {
		return nil, err
	}

	// Wait for LRO to complete
	err = updateDatabaseDdlOperation.Wait(ctx)
	if err != nil {
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

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Decompose name to get project, instance, database
	nameParts := strings.Split(name, "/")
	project := nameParts[1]
	instance := nameParts[3]
	databaseId := nameParts[5]

	var role *databasepb.DatabaseRole
	it := client.ListDatabaseRoles(ctx, &databasepb.ListDatabaseRolesRequest{
		Parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
	})
	for {
		r, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}

		if r.GetName() == name {
			role = r
			break
		}
	}

	if role == nil {
		return nil, status.Errorf(codes.NotFound, "Database role (%s) not found", name)
	}

	return role, nil
}

func (s *SpannerService) ListDatabaseRoles(ctx context.Context, parent string, pageSize int32, pageToken string) ([]*databasepb.DatabaseRole, string, error) {
	// Validate parent
	googleSqlValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlValid && !postgresSqlValid {
		return nil, "", status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlDatabaseNameRegex, utils.SpannerPostgresSqlDatabaseNameRegex)
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, "", err
	}
	defer client.Close()

	var res []*databasepb.DatabaseRole
	var nextPageToken string

	it := client.ListDatabaseRoles(ctx, &databasepb.ListDatabaseRolesRequest{
		Parent:    parent,
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	for {
		r, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		res = append(res, r)

		// Check if page size is reached
		if pageSize > 0 && len(res) >= int(pageSize) {
			nextPageToken = it.PageInfo().Token
			break
		}
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

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	// Decompose name to get project, instance, database
	nameParts := strings.Split(name, "/")
	project := nameParts[1]
	instance := nameParts[3]
	databaseId := nameParts[5]
	roleId := nameParts[7]

	// Get database state
	database, err := client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
	})
	if err != nil {
		return err
	}

	// CREATE ROLE inventory_admin;
	var ddlStatements []string
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("DROP ROLE %s", roleId))
	}
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("DROP ROLE %s", roleId))
	}
	updateDatabaseDdlOperation, err := client.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   database.GetName(),
		Statements: ddlStatements,
	})
	if err != nil {
		return err
	}

	// Wait for LRO to complete
	err = updateDatabaseDdlOperation.Wait(ctx)
	if err != nil {
		return err
	}

	return nil
}
