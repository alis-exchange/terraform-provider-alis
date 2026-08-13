package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// CreateSpannerSequence creates a sequence in the parent database via CREATE
// SEQUENCE DDL, applying the options carried on sequence. The database's
// existence is verified first so a missing database surfaces as its own error
// rather than a DDL failure.
func (s *SpannerService) CreateSpannerSequence(ctx context.Context, parent string, sequence *schema.SpannerSequence) (*schema.SpannerSequence, error) {
	// Validate arguments
	// Validate parent
	googleSqlParentValid := utils.ValidateArgument(parent, utils.SpannerGoogleSqlDatabaseNameRegex)
	postgresSqlParentValid := utils.ValidateArgument(parent, utils.SpannerPostgresSqlDatabaseNameRegex)
	if !googleSqlParentValid && !postgresSqlParentValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument parent (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", parent, utils.SpannerGoogleSqlTableNameRegex, utils.SpannerPostgresSqlTableNameRegex)
	}

	// Ensure sequence is provided
	if sequence.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument sequence.name, field is required but not provided")
	}

	// Verify the database exists before issuing DDL
	if _, err := s.conn.Dialect(ctx, parent); err != nil {
		return nil, err
	}

	// CREATE SEQUENCE MySequence;
	ddl, err := sequence.CreateDdl()
	if err != nil {
		return nil, err
	}
	if err := s.conn.ExecuteDDL(ctx, parent, ddl); err != nil {
		return nil, err
	}

	return &schema.SpannerSequence{
		Name:    fmt.Sprintf("%s/sequences/%s", parent, sequence.GetName()),
		Options: sequence.GetOptions(),
	}, nil
}

// GetSpannerSequence reads a sequence and its options (sequence_kind,
// skip_range, start_with_counter) back from INFORMATION_SCHEMA.SEQUENCES and
// SEQUENCE_OPTIONS. codes.NotFound is returned when the sequence does not
// exist; Options is nil when the sequence has no options set.
func (s *SpannerService) GetSpannerSequence(ctx context.Context, name string) (*schema.SpannerSequence, error) {
	// Validate arguments
	// Validate parent
	googleSqlNameValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlSequenceNameRegex)
	postgresSqlNameValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlSequenceNameRegex)
	if !googleSqlNameValid && !postgresSqlNameValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlSequenceNameRegex, utils.SpannerPostgresSqlSequenceNameRegex)
	}

	sequenceName, err := names.ParseSequence(name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s): %v", name, err)
	}
	sequenceId := sequenceName.Sequence
	database := sequenceName.DatabaseName().String()

	var rows []*SequenceRow
	if err := s.conn.Query(ctx, database, &rows,
		"SELECT s.CATALOG, s.SCHEMA, s.NAME AS SEQUENCE_NAME, s.DATA_TYPE, o.OPTION_NAME, o.OPTION_VALUE, o.OPTION_TYPE FROM INFORMATION_SCHEMA.SEQUENCES s LEFT JOIN INFORMATION_SCHEMA.SEQUENCE_OPTIONS o ON s.CATALOG = o.CATALOG AND s.SCHEMA = o.SCHEMA AND s.NAME = o.NAME WHERE s.NAME = ?", sequenceId); err != nil {
		return nil, status.Errorf(codes.Internal, "Error getting sequence: %v", err)
	}

	if len(rows) == 0 {
		return nil, status.Errorf(codes.NotFound, "Sequence %s not found", sequenceId)
	}

	// From sequence rows,

	var sequenceOptions *schema.SpannerSequenceOptions
	for _, row := range rows {
		// 1. Clean Guard Clause: Skip LEFT JOIN rows that have no actual options
		if row.OptionName == nil || *row.OptionName == "" || row.OptionValue == nil || *row.OptionValue == "" {
			continue
		}

		// Initialize the main options struct only once we know we have data
		if sequenceOptions == nil {
			sequenceOptions = &schema.SpannerSequenceOptions{}
		}

		optionName := *row.OptionName
		optionType := ""
		if row.OptionType != nil {
			optionType = *row.OptionType
		}

		// 2. Strip SQL literal quotes from Spanner strings (leaves numbers untouched)
		optionValue := strings.Trim(*row.OptionValue, `"'`)

		switch optionName {
		case "sequence_kind":
			if optionType != "STRING" {
				continue
			}

			sequenceOptions.SequenceKind = schema.SpannerSequenceKindFromString(optionValue)

		case "skip_range_min":
			if optionType != "INT64" {
				continue
			}

			// 3. Fix: Prevent nil pointer panic by initializing the nested struct
			if sequenceOptions.SkipRange == nil {
				sequenceOptions.SkipRange = &schema.SpannerSequenceSkipRange{}
			}

			min, err := strconv.ParseInt(optionValue, 10, 64)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Error parsing skip_range_min: %v", err)
			}
			sequenceOptions.SkipRange.Min = wrapperspb.Int64(min)

		case "skip_range_max":
			if optionType != "INT64" {
				continue
			}

			// 3. Fix: Prevent nil pointer panic
			if sequenceOptions.SkipRange == nil {
				sequenceOptions.SkipRange = &schema.SpannerSequenceSkipRange{}
			}

			max, err := strconv.ParseInt(optionValue, 10, 64)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Error parsing skip_range_max: %v", err)
			}
			sequenceOptions.SkipRange.Max = wrapperspb.Int64(max)

		case "start_with_counter":
			if optionType != "INT64" {
				continue
			}

			counter, err := strconv.ParseInt(optionValue, 10, 64)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Error parsing start_with_counter: %v", err)
			}
			sequenceOptions.StartWithCounter = wrapperspb.Int64(counter)
		}
	}

	return &schema.SpannerSequence{
		Name:    sequenceName.String(),
		Options: sequenceOptions,
	}, nil
}

// UpdateSpannerSequence applies the sequence's options in place via ALTER
// SEQUENCE ... SET OPTIONS.
func (s *SpannerService) UpdateSpannerSequence(ctx context.Context, sequence *schema.SpannerSequence) (*schema.SpannerSequence, error) {
	// Ensure sequence is provided
	if sequence.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid argument sequence.name, field is required but not provided")
	}

	// Validate arguments
	// Validate name
	googleSqlNameValid := utils.ValidateArgument(sequence.GetName(), utils.SpannerGoogleSqlSequenceNameRegex)
	postgresSqlNameValid := utils.ValidateArgument(sequence.GetName(), utils.SpannerPostgresSqlSequenceNameRegex)
	if !googleSqlNameValid && !postgresSqlNameValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", sequence.GetName(), utils.SpannerGoogleSqlSequenceNameRegex, utils.SpannerPostgresSqlSequenceNameRegex)
	}

	sequenceName, err := names.ParseSequence(sequence.GetName())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument sequence.name (%s): %v", sequence.GetName(), err)
	}
	database := sequenceName.DatabaseName().String()

	// Verify the database exists before issuing DDL
	if _, err := s.conn.Dialect(ctx, database); err != nil {
		return nil, err
	}

	ddl, err := sequence.AlterDdl()
	if err != nil {
		return nil, err
	}
	if err := s.conn.ExecuteDDL(ctx, database, ddl); err != nil {
		return nil, err
	}

	return sequence, nil
}

// DeleteSpannerSequence drops the sequence via DROP SEQUENCE DDL.
func (s *SpannerService) DeleteSpannerSequence(ctx context.Context, name string) error {
	// Validate arguments
	// Validate name
	googleSqlNameValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlSequenceNameRegex)
	postgresSqlNameValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlSequenceNameRegex)
	if !googleSqlNameValid && !postgresSqlNameValid {
		return status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlSequenceNameRegex, utils.SpannerPostgresSqlSequenceNameRegex)
	}

	sequenceName, err := names.ParseSequence(name)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "Invalid argument name (%s): %v", name, err)
	}
	database := sequenceName.DatabaseName().String()

	// Verify the database exists before issuing DDL
	if _, err := s.conn.Dialect(ctx, database); err != nil {
		return err
	}

	sequence := &schema.SpannerSequence{
		Name: name,
	}

	ddl, err := sequence.DropDdl()
	if err != nil {
		return err
	}
	return s.conn.ExecuteDDL(ctx, database, ddl)
}
