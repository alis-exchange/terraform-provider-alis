package services

import (
	"context"
	"regexp"
	"strconv"

	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// rowDeletionPolicyExpression captures the column and day count out of the
// OLDER_THAN expression INFORMATION_SCHEMA returns for a TTL policy. Every
// plan and refresh parses one, so it is compiled once here.
var rowDeletionPolicyExpression = regexp.MustCompile(`OLDER_THAN\((\w+),\s*INTERVAL\s+(\d+)\s+DAY\)`)

// CreateSpannerTableRowDeletionPolicy adds a row deletion (TTL) policy to the
// table named by parent. ttl.Duration is the number of days past ttl.Column
// after which rows become eligible for deletion. The parent table must exist;
// Spanner allows at most one policy per table.
func (s *SpannerService) CreateSpannerTableRowDeletionPolicy(
	ctx context.Context,
	parent string,
	ttl *SpannerTableRowDeletionPolicy,
) (*SpannerTableRowDeletionPolicy, error) {
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSqlTableNameRegex,
		utils.SpannerPostgresSqlTableNameRegex,
	); err != nil {
		return nil, err
	}
	// Ensure ttl is provided and has a name, column and duration
	if ttl == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl, field is required but not provided")
	}
	if ttl.Column == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.column, field is required but not provided")
	}
	if err := utils.ValidateDialectArgument(
		"ttl.column",
		ttl.Column,
		utils.SpannerGoogleSqlColumnIdRegex,
		utils.SpannerPostgresSqlColumnIdRegex,
	); err != nil {
		return nil, err
	}
	if ttl.Duration == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.duration, field is required but not provided")
	}
	if ttl.Duration.GetValue() < 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.duration, field must be greater than or equal to 0")
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableId := parentName.Table

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

// GetSpannerTableRowDeletionPolicy reads the table's row deletion policy by
// parsing the OLDER_THAN(column, INTERVAL n DAY) expression from
// INFORMATION_SCHEMA.TABLES. codes.NotFound is returned when the table has no
// policy.
func (s *SpannerService) GetSpannerTableRowDeletionPolicy(ctx context.Context, parent string) (*SpannerTableRowDeletionPolicy, error) {
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSqlTableNameRegex,
		utils.SpannerPostgresSqlTableNameRegex,
	); err != nil {
		return nil, err
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableId := parentName.Table

	// Get parent table
	if _, err := s.GetSpannerTable(ctx, parent); err != nil {
		return nil, err
	}

	type RowDeletionPolicy struct {
		TABLE_NAME                     string
		ROW_DELETION_POLICY_EXPRESSION string
	}
	var policy RowDeletionPolicy
	if err := s.conn.Query(
		ctx,
		database,
		&policy,
		"SELECT TABLE_NAME, ROW_DELETION_POLICY_EXPRESSION FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = ? AND ROW_DELETION_POLICY_EXPRESSION IS NOT NULL",
		tableId,
	); err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, status.Errorf(codes.NotFound, "Row deletion policy not found")
		}
		return nil, status.Errorf(codes.Internal, "Error getting row deletion policy: %v", err)
	}
	if policy.ROW_DELETION_POLICY_EXPRESSION == "" {
		return nil, status.Errorf(codes.NotFound, "Row deletion policy not found")
	}

	// Find all matches and capture groups
	matches := rowDeletionPolicyExpression.FindStringSubmatch(policy.ROW_DELETION_POLICY_EXPRESSION)

	if len(matches) != 3 {
		return nil, status.Errorf(
			codes.Internal,
			"Error parsing row deletion policy: unexpected expression %q",
			policy.ROW_DELETION_POLICY_EXPRESSION,
		)
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

// UpdateSpannerTableRowDeletionPolicy replaces the table's existing row
// deletion policy via ALTER TABLE ... REPLACE ROW DELETION POLICY.
func (s *SpannerService) UpdateSpannerTableRowDeletionPolicy(
	ctx context.Context,
	parent string,
	ttl *SpannerTableRowDeletionPolicy,
) (*SpannerTableRowDeletionPolicy, error) {
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSqlTableNameRegex,
		utils.SpannerPostgresSqlTableNameRegex,
	); err != nil {
		return nil, err
	}
	// Ensure ttl is provided and has a name, column and duration
	if ttl == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl, field is required but not provided")
	}
	if ttl.Column == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.column, field is required but not provided")
	}
	if err := utils.ValidateDialectArgument(
		"ttl.column",
		ttl.Column,
		utils.SpannerGoogleSqlColumnIdRegex,
		utils.SpannerPostgresSqlColumnIdRegex,
	); err != nil {
		return nil, err
	}
	if ttl.Duration == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.duration, field is required but not provided")
	}
	if ttl.Duration.GetValue() < 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument ttl.duration, field must be greater than or equal to 0")
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableId := parentName.Table

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

// DeleteSpannerTableRowDeletionPolicy drops the table's row deletion policy.
func (s *SpannerService) DeleteSpannerTableRowDeletionPolicy(ctx context.Context, parent string) error {
	if err := utils.ValidateDialectArgument(
		"parent",
		parent,
		utils.SpannerGoogleSqlTableNameRegex,
		utils.SpannerPostgresSqlTableNameRegex,
	); err != nil {
		return err
	}

	parentName, err := names.ParseTable(parent)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s): %v", parent, err)
	}
	database := parentName.DatabaseName().String()
	tableId := parentName.Table

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
