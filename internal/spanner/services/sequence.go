package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"terraform-provider-alis/internal/spanner/schema"
	"terraform-provider-alis/internal/utils"

	spanner "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	spannergorm "github.com/googleapis/go-gorm-spanner"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gorm.io/gorm"
)

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

	// CREATE SEQUENCE MySequence;
	ddl, err := sequence.CreateDdl()
	if err != nil {
		return nil, err
	}
	var ddlStatements []string
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
		ddlStatements = append(ddlStatements, ddl)
	}
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
		ddlStatements = append(ddlStatements, ddl)
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

	return &schema.SpannerSequence{
		Name:    fmt.Sprintf("%s/sequences/%s", parent, sequence.GetName()),
		Options: sequence.GetOptions(),
	}, nil
}

func (s *SpannerService) GetSpannerSequence(ctx context.Context, name string) (*schema.SpannerSequence, error) {
	// Validate arguments
	// Validate parent
	googleSqlNameValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlSequenceNameRegex)
	postgresSqlNameValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlSequenceNameRegex)
	if !googleSqlNameValid && !postgresSqlNameValid {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlSequenceNameRegex, utils.SpannerPostgresSqlSequenceNameRegex)
	}

	// "projects/my-project/instances/my-instance/database/my-db/sequences/my-sequence"
	sequenceNameParts := strings.Split(name, "/")
	projectId := sequenceNameParts[1]
	instanceId := sequenceNameParts[3]
	databaseId := sequenceNameParts[5]
	sequenceId := sequenceNameParts[7]

	db, err := gorm.Open(
		spannergorm.New(
			spannergorm.Config{
				DriverName: "spanner",
				DSN:        fmt.Sprintf("projects/%s/instances/%s/databases/%s", projectId, instanceId, databaseId),
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

	// 	SELECT
	//     s.NAME AS SEQUENCE_NAME,
	//     ARRAY(
	//         SELECT AS STRUCT
	//             o.OPTION_NAME,
	//             o.OPTION_VALUE,
	//             o.OPTION_TYPE
	//         FROM
	//             INFORMATION_SCHEMA.SEQUENCE_OPTIONS o
	//         WHERE
	//             o.CATALOG = s.CATALOG
	//             AND o.SCHEMA = s.SCHEMA
	//             AND o.NAME = s.NAME
	//     ) AS OPTIONS
	// FROM
	//     INFORMATION_SCHEMA.SEQUENCES s
	// WHERE
	//     s.NAME = 'test_sequence';
	var rows []*SequenceRow
	res := db.Raw("SELECT s.CATALOG, s.SCHEMA, s.NAME AS SEQUENCE_NAME, s.DATA_TYPE, o.OPTION_NAME, o.OPTION_VALUE, o.OPTION_TYPE FROM INFORMATION_SCHEMA.SEQUENCES s LEFT JOIN INFORMATION_SCHEMA.SEQUENCE_OPTIONS o ON s.CATALOG = o.CATALOG AND s.SCHEMA = o.SCHEMA AND s.NAME = o.NAME WHERE s.NAME = ?", sequenceId).Scan(&rows)
	if res.Error != nil {
		return nil, status.Errorf(codes.Internal, "Error getting sequence: %v", res.Error)
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
		Name:    fmt.Sprintf("projects/%s/instances/%s/databases/%s/sequences/%s", projectId, instanceId, databaseId, sequenceId),
		Options: sequenceOptions,
	}, nil
}

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

	// "projects/my-project/instances/my-instance/database/my-db/sequences/my-sequence"
	sequenceNameParts := strings.Split(sequence.GetName(), "/")
	projectId := sequenceNameParts[1]
	instanceId := sequenceNameParts[3]
	databaseId := sequenceNameParts[5]

	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Get database state
	database, err := client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", projectId, instanceId, databaseId),
	})
	if err != nil {
		return nil, err
	}

	ddl, err := sequence.AlterDdl()
	if err != nil {
		return nil, err
	}
	var ddlStatements []string
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
		ddlStatements = append(ddlStatements, ddl)
	}
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
		ddlStatements = append(ddlStatements, ddl)
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

	return sequence, nil
}

func (s *SpannerService) DeleteSpannerSequence(ctx context.Context, name string) error {
	// Validate arguments
	// Validate name
	googleSqlNameValid := utils.ValidateArgument(name, utils.SpannerGoogleSqlSequenceNameRegex)
	postgresSqlNameValid := utils.ValidateArgument(name, utils.SpannerPostgresSqlSequenceNameRegex)
	if !googleSqlNameValid && !postgresSqlNameValid {
		return status.Errorf(codes.InvalidArgument, "Invalid argument name (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect", name, utils.SpannerGoogleSqlSequenceNameRegex, utils.SpannerPostgresSqlSequenceNameRegex)
	}

	// "projects/my-project/instances/my-instance/database/my-db/sequences/my-sequence"
	sequenceNameParts := strings.Split(name, "/")
	projectId := sequenceNameParts[1]
	instanceId := sequenceNameParts[3]
	databaseId := sequenceNameParts[5]

	client, err := spanner.NewDatabaseAdminClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	// Get database state
	database, err := client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", projectId, instanceId, databaseId),
	})
	if err != nil {
		return err
	}

	sequence := &schema.SpannerSequence{
		Name: name,
	}

	ddl, err := sequence.DropDdl()
	if err != nil {
		return err
	}
	var ddlStatements []string
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL {
		ddlStatements = append(ddlStatements, ddl)
	}
	if database.GetDatabaseDialect() == databasepb.DatabaseDialect_POSTGRESQL {
		ddlStatements = append(ddlStatements, ddl)
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
