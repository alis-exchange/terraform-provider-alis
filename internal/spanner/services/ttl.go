package services

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"terraform-provider-alis/internal/utils"

	spannergorm "github.com/googleapis/go-gorm-spanner"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gorm.io/gorm"
)

func (s *SpannerService) CreateSpannerTableRowDeletionPolicy(ctx context.Context, parent string, ttl *SpannerTableRowDeletionPolicy) (*SpannerTableRowDeletionPolicy, error) {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}
	// Ensure ttl is provided and has a name, column and duration
	if ttl == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl, field is required but not provided")
	}
	if ttl.Column == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.column, field is required but not provided")
	}
	googleSqlColumnIdValid := utils.ValidateArgument(ttl.Column, utils.SpannerGoogleSqlColumnIdRegex)
	postgresSqlColumnIdValid := utils.ValidateArgument(ttl.Column, utils.SpannerPostgresSqlColumnIdRegex)
	if !googleSqlColumnIdValid && !postgresSqlColumnIdValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument ttl.column (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", ttl.Column, utils.SpannerGoogleSqlColumnIdRegex, utils.SpannerPostgresSqlColumnIdRegex)
	}
	if ttl.Duration == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.duration, field is required but not provided")
	}
	if ttl.Duration.GetValue() < 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.duration, field must be greater than or equal to 0")
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

	// Get parent table
	_, err = s.GetSpannerTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	// Create the deletion policy
	if err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD ROW DELETION POLICY (OLDER_THAN(%s, INTERVAL %d DAY))", tableId, ttl.Column, ttl.Duration.GetValue())).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating row deletion policy: %v", err)
	}

	return ttl, nil
}

func (s *SpannerService) GetSpannerTableRowDeletionPolicy(ctx context.Context, parent string) (*SpannerTableRowDeletionPolicy, error) {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
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

	// Get parent table
	_, err = s.GetSpannerTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	type RowDeletionPolicy struct {
		TABLE_NAME                     string
		ROW_DELETION_POLICY_EXPRESSION string
	}
	var policy *RowDeletionPolicy
	err = db.Raw("SELECT TABLE_NAME, ROW_DELETION_POLICY_EXPRESSION FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = ? AND ROW_DELETION_POLICY_EXPRESSION IS NOT NULL", tableId).Scan(&policy).Error
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error getting row deletion policy: %v", err)
	}

	if policy == nil || policy.ROW_DELETION_POLICY_EXPRESSION == "" {
		return nil, status.Errorf(codes.NotFound, "Row deletion policy not found")
	}

	// Regular expression with capture groups
	re := regexp.MustCompile(`OLDER_THAN\((\w+),\s*INTERVAL\s+(\d+)\s+DAY\)`)

	// Find all matches and capture groups
	matches := re.FindStringSubmatch(policy.ROW_DELETION_POLICY_EXPRESSION)

	if len(matches) != 3 {
		return nil, status.Errorf(codes.Internal, "Error parsing row deletion policy: %v", err)
	}

	column := matches[1]
	durationStr := matches[2]
	duration, err := strconv.ParseInt(durationStr, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error parsing row deletion policy: %v", err)
	}

	return &SpannerTableRowDeletionPolicy{
		Column:   column,
		Duration: wrapperspb.Int64(duration),
	}, nil
}

func (s *SpannerService) UpdateSpannerTableRowDeletionPolicy(ctx context.Context, parent string, ttl *SpannerTableRowDeletionPolicy) (*SpannerTableRowDeletionPolicy, error) {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}
	// Ensure ttl is provided and has a name, column and duration
	if ttl == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl, field is required but not provided")
	}
	if ttl.Column == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.column, field is required but not provided")
	}
	googleSqlColumnIdValid := utils.ValidateArgument(ttl.Column, utils.SpannerGoogleSqlColumnIdRegex)
	postgresSqlColumnIdValid := utils.ValidateArgument(ttl.Column, utils.SpannerPostgresSqlColumnIdRegex)
	if !googleSqlColumnIdValid && !postgresSqlColumnIdValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument ttl.column (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", ttl.Column, utils.SpannerGoogleSqlColumnIdRegex, utils.SpannerPostgresSqlColumnIdRegex)
	}
	if ttl.Duration == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.duration, field is required but not provided")
	}
	if ttl.Duration.GetValue() < 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.duration, field must be greater than or equal to 0")
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

	// Get parent table
	_, err = s.GetSpannerTable(ctx, parent)
	if err != nil {
		return nil, err
	}

	// Create the deletion policy
	if err := db.Exec(fmt.Sprintf("ALTER TABLE %s REPLACE ROW DELETION POLICY (OLDER_THAN(%s, INTERVAL %d DAY))", tableId, ttl.Column, ttl.Duration.GetValue())).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating row deletion policy: %v", err)
	}

	return ttl, nil
}

func (s *SpannerService) DeleteSpannerTableRowDeletionPolicy(ctx context.Context, parent string) error {
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlTableNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlTableNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
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
		return status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}
	db = db.WithContext(ctx)

	// Get parent table
	_, err = s.GetSpannerTable(ctx, parent)
	if err != nil {
		return err
	}

	// Create the deletion policy
	if err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP ROW DELETION POLICY", tableId)).Error; err != nil {
		return status.Errorf(codes.Internal, "Error creating row deletion policy: %v", err)
	}

	return nil
}
