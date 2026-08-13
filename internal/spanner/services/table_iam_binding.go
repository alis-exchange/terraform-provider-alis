package services

import (
	"context"
	"fmt"
	"strings"

	"terraform-provider-alis/internal/utils"

	spanner "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	spannergorm "github.com/googleapis/go-gorm-spanner"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
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
	if binding.Permissions == nil || len(binding.Permissions) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument binding.permissions, field is required but not provided")
	}

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Deconstruct parent name to get project, instance, database and table
	parentNameParts := strings.Split(parent, "/")
	project := parentNameParts[1]
	instance := parentNameParts[3]
	databaseId := parentNameParts[5]
	tableId := parentNameParts[7]

	// Get database state
	database, err := client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
	})
	if err != nil {
		return nil, err
	}

	var permissions []string
	for _, permission := range binding.Permissions {
		permissions = append(permissions, permission.String())
	}

	// CREATE ROLE inventory_admin;
	var ddlStatements []string
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("GRANT %s ON TABLE %s TO ROLE %s", strings.Join(permissions, ", "), tableId, binding.Role))
	}
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("GRANT %s ON TABLE %s TO ROLE %s", strings.Join(permissions, ", "), tableId, binding.Role))
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

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
			},
		),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      tfLogger,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	var rows []*TablePermissionsRow
	res := db.Raw("SELECT * FROM INFORMATION_SCHEMA.TABLE_PRIVILEGES WHERE table_name = ? AND grantee = ?", tableId, role).Scan(&rows)
	if res.Error != nil {
		return nil, status.Errorf(codes.Internal, "Error getting table IAM binding: %v", res.Error)
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

	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	// Get database state
	database, err := client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId),
	})
	if err != nil {
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

	// CREATE ROLE inventory_admin;
	var ddlStatements []string
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("REVOKE %s ON TABLE %s FROM ROLE %s", strings.Join(permissions, ", "), tableId, role))
	}
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
		ddlStatements = append(ddlStatements, fmt.Sprintf("REVOKE %s ON TABLE %s FROM ROLE %s", strings.Join(permissions, ", "), tableId, role))
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
