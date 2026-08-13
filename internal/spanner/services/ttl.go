package services

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"
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
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId)

	// Get parent table
	if _, err := s.GetSpannerTable(ctx, parent); err != nil {
		return nil, err
	}

	// Create the deletion policy
	ddl, err := ttl.CreateDdl(tableId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := s.conn.ExecuteDDL(ctx, database, ddl); err != nil {
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
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId)

	// Get parent table
	if _, err := s.GetSpannerTable(ctx, parent); err != nil {
		return nil, err
	}

	type RowDeletionPolicy struct {
		TABLE_NAME                     string
		ROW_DELETION_POLICY_EXPRESSION string
	}
	var policy RowDeletionPolicy
	if err := s.conn.Query(ctx, database, &policy,
		"SELECT TABLE_NAME, ROW_DELETION_POLICY_EXPRESSION FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = ? AND ROW_DELETION_POLICY_EXPRESSION IS NOT NULL", tableId); err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, status.Errorf(codes.NotFound, "Row deletion policy not found")
		}
		return nil, status.Errorf(codes.Internal, "Error getting row deletion policy: %v", err)
	}
	if policy.ROW_DELETION_POLICY_EXPRESSION == "" {
		return nil, status.Errorf(codes.NotFound, "Row deletion policy not found")
	}

	// Regular expression with capture groups
	re := regexp.MustCompile(`OLDER_THAN\((\w+),\s*INTERVAL\s+(\d+)\s+DAY\)`)

	// Find all matches and capture groups
	matches := re.FindStringSubmatch(policy.ROW_DELETION_POLICY_EXPRESSION)

	if len(matches) != 3 {
		return nil, status.Errorf(codes.Internal, "Error parsing row deletion policy: unexpected expression %q", policy.ROW_DELETION_POLICY_EXPRESSION)
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
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId)

	// Get parent table
	if _, err := s.GetSpannerTable(ctx, parent); err != nil {
		return nil, err
	}

	// Replace the deletion policy
	ddl, err := ttl.ReplaceDdl(tableId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := s.conn.ExecuteDDL(ctx, database, ddl); err != nil {
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
	database := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, databaseId)

	// Get parent table
	if _, err := s.GetSpannerTable(ctx, parent); err != nil {
		return err
	}

	// Drop the deletion policy
	if err := s.conn.ExecuteDDL(ctx, database, schema.DropRowDeletionPolicyDdl(tableId)); err != nil {
		return status.Errorf(codes.Internal, "Error creating row deletion policy: %v", err)
	}

	return nil
}
