package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/spanner"
	spannerAdmin "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	_ "github.com/googleapis/go-sql-spanner"
	dynamicstruct "github.com/ompluscator/dynamic-struct"
	alUtils "go.alis.build/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	customdatatypes "terraform-provider-alis/internal/spanner/datatypes"
	"terraform-provider-alis/internal/utils"
)

func GetIndexes(db *gorm.DB, tableName string) ([]*SpannerTableIndex, error) {

	currentDatabase := db.Migrator().CurrentDatabase()
	// Get the indexes for the table
	var results []*Index
	db = db.Raw(
		"SELECT i.index_name,"+
			"i.is_unique,"+
			"i.index_type,"+
			"ic.ordinal_position,"+
			"ic.column_ordering,"+
			"ic.is_nullable,"+
			"col.column_name"+
			" FROM information_schema.indexes i"+
			" LEFT JOIN information_schema.index_columns ic ON ic.table_name = i.table_name AND ic.index_name = i.index_name"+
			" LEFT JOIN information_schema.columns col ON col.column_name = ic.column_name AND col.table_name = ic.table_name"+
			" WHERE i.index_name IS NOT NULL AND i.table_schema = ? AND i.table_name = ?",
		currentDatabase, tableName,
	).Scan(&results)

	resultsMap := map[string]map[string]*Index{}
	for _, r := range results {
		if _, ok := resultsMap[r.IndexName]; !ok {
			resultsMap[r.IndexName] = map[string]*Index{}
		}
		resultsMap[r.IndexName][r.ColumnName] = r
	}

	indexMap := make(map[string]*SpannerTableIndex)
	for _, r := range results {
		if r.IndexType == "PRIMARY_KEY" {
			continue
		}

		idx, ok := indexMap[r.IndexName]
		if !ok {
			idx = &SpannerTableIndex{
				Name:    r.IndexName,
				Columns: []*SpannerTableIndexColumn{},
				Unique:  wrapperspb.Bool(r.IsUnique),
			}
		}
		var order SpannerTableIndexColumnOrder
		switch r.ColumnOrdering {
		case "ASC":
			order = SpannerTableIndexColumnOrder_ASC
		case "DESC":
			order = SpannerTableIndexColumnOrder_DESC
		}
		idx.Columns = append(idx.Columns, &SpannerTableIndexColumn{
			Name:  r.ColumnName,
			Order: order,
		})
		indexMap[r.IndexName] = idx
	}

	indexes := make([]*SpannerTableIndex, 0)
	for _, idx := range indexMap {
		// Sort the columns by ordinal position
		sort.Slice(idx.Columns, func(i, j int) bool {
			return resultsMap[idx.Name][idx.Columns[i].Name].OrdinalPosition < resultsMap[idx.Name][idx.Columns[j].Name].OrdinalPosition
		})

		// Append the index to the list
		indexes = append(indexes, idx)
	}

	return indexes, nil
}

func CreateIndex(db *gorm.DB, tableName string, index *SpannerTableIndex) error {
	unique := ""
	if index.Unique != nil && index.Unique.GetValue() {
		unique = "UNIQUE"
	}
	columns := make([]string, 0)
	for _, column := range index.Columns {

		if column.Order == SpannerTableIndexColumnOrder_UNSPECIFIED {
			column.Order = SpannerTableIndexColumnOrder_ASC
		}

		columns = append(columns, fmt.Sprintf("%s %s", column.Name, column.Order.String()))
	}

	// Create the index
	if err := db.Exec(fmt.Sprintf("CREATE %s INDEX %s ON %s (%s)",
		unique,
		index.Name,
		tableName,
		strings.Join(columns, ", "),
	)).Error; err != nil {
		return err
	}

	return nil
}

func GetColumnMetadata(db *gorm.DB, tableName string) ([]*ColumnMetadata, error) {
	// Create or Update ColumnMetadata table
	if err := db.AutoMigrate(&ColumnMetadata{}); err != nil {
		return nil, err
	}

	// Get ColumnMetadata table
	var results []*ColumnMetadata
	if err := db.Where("table_name = ?", tableName).Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
func UpdateColumnMetadata(db *gorm.DB, tableName string, columns []*SpannerTableColumn) error {
	// Create or Update ColumnMetadata table
	// IMPORTANT: When tables don't depend on each other, terraform will attempt to create them in parallel.
	// This can cause the migration to run at the same time for multiple tables, which can lead to a duplicate table error.
	// To prevent this, we'll retry the migration a few times.
	_, err := utils.Retry(5, 10*time.Second, func() (interface{}, error) {
		if err := db.AutoMigrate(&ColumnMetadata{}); err != nil {
			return nil, err
		}

		return nil, nil
	})
	if err != nil {
		return err
	}

	// Update ColumnMetadata table
	for _, column := range columns {

		meta := &ColumnMetadataMeta{}
		meta.Type = column.Type
		if column.Size != nil {
			meta.Size = fmt.Sprintf("%d", column.Size.GetValue())
		} else {
			meta.Size = "nil"
		}
		if column.Precision != nil {
			meta.Precision = fmt.Sprintf("%d", column.Precision.GetValue())
		} else {
			meta.Precision = "nil"
		}
		if column.Scale != nil {
			meta.Scale = fmt.Sprintf("%d", column.Scale.GetValue())
		} else {
			meta.Scale = "nil"
		}
		if column.Required != nil {
			meta.Required = fmt.Sprintf("%t", column.Required.GetValue())
		} else {
			meta.Required = "nil"
		}
		if column.AutoIncrement != nil {
			meta.AutoIncrement = fmt.Sprintf("%t", column.AutoIncrement.GetValue())
		} else {
			meta.AutoIncrement = "nil"
		}
		if column.Unique != nil {
			meta.Unique = fmt.Sprintf("%t", column.Unique.GetValue())
		} else {
			meta.Unique = "nil"
		}
		if column.AutoCreateTime != nil {
			meta.AutoCreateTime = fmt.Sprintf("%t", column.AutoCreateTime.GetValue())
		} else {
			meta.AutoCreateTime = "nil"
		}
		if column.AutoUpdateTime != nil {
			meta.AutoUpdateTime = fmt.Sprintf("%t", column.AutoUpdateTime.GetValue())
		} else {
			meta.AutoUpdateTime = "nil"
		}
		if column.DefaultValue != nil {
			meta.DefaultValue = column.DefaultValue.GetValue()
		} else {
			meta.DefaultValue = "nil"
		}
		if column.IsPrimaryKey != nil {
			meta.IsPrimaryKey = fmt.Sprintf("%t", column.IsPrimaryKey.GetValue())
		} else {
			meta.IsPrimaryKey = "nil"
		}
		if column.IsComputed != nil {
			meta.IsComputed = fmt.Sprintf("%t", column.IsComputed.GetValue())
		} else {
			meta.IsComputed = "nil"
		}
		if column.ComputationDdl != nil {
			meta.ComputationDdl = column.ComputationDdl.GetValue()
		} else {
			meta.ComputationDdl = "nil"
		}
		if column.ProtoFileDescriptorSet != nil {
			if column.ProtoFileDescriptorSet.ProtoPackage != nil {
				meta.ProtoPackage = column.ProtoFileDescriptorSet.ProtoPackage.GetValue()
			} else {
				meta.ProtoPackage = "nil"
			}
			if column.ProtoFileDescriptorSet.FileDescriptorSetPath != nil {
				meta.FileDescriptorSetPath = column.ProtoFileDescriptorSet.FileDescriptorSetPath.GetValue()
			} else {
				meta.FileDescriptorSetPath = "nil"
			}
			if column.ProtoFileDescriptorSet.FileDescriptorSetPathSource != ProtoFileDescriptorSetSourceUNSPECIFIED {
				meta.FileDescriptorSetPathSource = fmt.Sprintf("%d", column.ProtoFileDescriptorSet.FileDescriptorSetPathSource)
			} else {
				meta.FileDescriptorSetPathSource = "nil"
			}
		}

		createRes := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&ColumnMetadata{
			TableName:  tableName,
			ColumnName: column.Name,
			Metadata:   meta,
		})
		if createRes.Error != nil {
			if status.Code(createRes.Error) != codes.AlreadyExists {
				return createRes.Error
			}

			updateRes := db.Clauses(clause.OnConflict{UpdateAll: true}).Save(&ColumnMetadata{
				TableName:  tableName,
				ColumnName: column.Name,
				Metadata:   meta,
			})
			if updateRes.Error != nil {
				return updateRes.Error
			}
		}
	}

	return nil
}
func DeleteColumnMetadata(db *gorm.DB, tableName string, columns []*SpannerTableColumn) error {
	// Create or Update ColumnMetadata table
	if err := db.AutoMigrate(&ColumnMetadata{}); err != nil {
		return err
	}

	// Delete ColumnMetadata table
	if columns == nil || len(columns) == 0 {
		deleteRes := db.Where("table_name = ?", tableName).Delete(&ColumnMetadata{})
		if deleteRes.Error != nil {
			return deleteRes.Error
		}
	} else {
		for _, column := range columns {
			deleteRes := db.Where("table_name = ? AND column_name = ?", tableName, column.Name).Delete(&ColumnMetadata{})
			if deleteRes.Error != nil {
				return deleteRes.Error
			}
		}
	}

	return nil
}

// ParseSchemaToStruct parses a *v1.SpannerTable_Schema to a struct that can be used with gorm
func ParseSchemaToStruct(schema *SpannerTableSchema) (interface{}, error) {
	// Ensure the schema is not nil
	if schema == nil {
		return nil, errors.New("schema is nil")
	}
	// Ensure the schema has columns
	if len(schema.Columns) == 0 {
		return nil, errors.New("schema requires at least one column")
	}

	// Create a new dynamic struct
	instance := dynamicstruct.NewStruct()

	// Iterate over the columns and add them to the struct
	for _, column := range schema.Columns {
		// `gorm:"column"`
		// `gorm:"primaryKey"`
		// `gorm:"unique"`
		// `gorm:"default"`
		// `gorm:"precision"`
		// `gorm:"scale"`
		// `gorm:"not null"`
		// `gorm:"autoCreateTime"`
		// `gorm:"autoUpdateTime"`
		// `gorm:"index"`
		// `gorm:"size"`
		gormTags := make([]string, 0)
		// Add column name
		gormTags = append(gormTags, fmt.Sprintf("column:%s", column.Name))
		// Check if the column is a primary key
		if column.IsPrimaryKey != nil && column.IsPrimaryKey.GetValue() {
			gormTags = append(gormTags, "primaryKey")
		}

		// Check if the column is a PROTO type column
		// Since we override the column type, the gorm generated DDL gets a bit distorted,
		// for this reason, we'll set the column type and other arguments manually using the `type` gorm tag
		if column.Type == SpannerTableDataType_PROTO.String() &&
			column.ProtoFileDescriptorSet != nil &&
			column.ProtoFileDescriptorSet.ProtoPackage != nil &&
			column.ProtoFileDescriptorSet.ProtoPackage.GetValue() != "" {
			// Set the column type manually since we'll be overriding it using the `type` gorm tag
			columnTypeArgs := ""

			{
				// Set the appropriate column type
				columnTypeArgs += column.ProtoFileDescriptorSet.ProtoPackage.GetValue()
			}

			{
				// Set nullable
				if column.Required != nil && column.Required.GetValue() {
					columnTypeArgs += " NOT NULL"
				}
			}

			gormTags = append(gormTags, fmt.Sprintf("type: %s", columnTypeArgs))
		} else if column.IsComputed != nil && column.IsComputed.GetValue() && column.ComputationDdl != nil && column.ComputationDdl.GetValue() != "" {
			// Check if the column is computed
			// Since we override the column type, the gorm generated DDL gets a bit distorted,
			// for this reason, we'll set the column type and other arguments manually using the `type` gorm tag

			// Set the column type manually since we'll be overriding it using the `type` gorm tag
			columnTypeArgs := ""

			{
				// Set the appropriate column type
				switch column.Type {
				// For PROTO column types we need to set the type as the proto package name
				case SpannerTableDataType_PROTO.String():
					if column.ProtoFileDescriptorSet != nil && column.ProtoFileDescriptorSet.ProtoPackage != nil {
						columnTypeArgs += column.ProtoFileDescriptorSet.ProtoPackage.GetValue()
					}
				default:
					columnTypeArgs += column.Type
				}
			}

			{
				// Set size for STRING and BYTES columns
				size := "MAX"
				if column.Size != nil {
					size = fmt.Sprintf("%d", column.Size.GetValue())
				}
				switch columnTypeArgs {
				case SpannerTableDataType_STRING.String(), SpannerTableDataType_BYTES.String():
					columnTypeArgs += fmt.Sprintf("(%s)", size)
				}
			}

			{
				// Set nullable
				if column.Required != nil && column.Required.GetValue() {
					columnTypeArgs += " NOT NULL"
				}
			}

			// TODO: Should we support default values for computed columns?

			gormTags = append(gormTags, fmt.Sprintf("type: %s AS (%s) STORED", columnTypeArgs, column.ComputationDdl.GetValue()))
		} else {
			// Set auto increment
			if column.AutoIncrement != nil && column.AutoIncrement.GetValue() {
				gormTags = append(gormTags, "autoIncrement:true")
			} else {
				gormTags = append(gormTags, "autoIncrement:false")
			}
			// Check if unique is set
			if column.Unique != nil && column.Unique.GetValue() {
				gormTags = append(gormTags, "unique")
			}
			// Set auto create
			if column.AutoCreateTime != nil && column.AutoCreateTime.GetValue() {
				gormTags = append(gormTags, "autoCreateTime:nano")
			}
			// Set auto update
			if column.AutoUpdateTime != nil && column.AutoUpdateTime.GetValue() {
				gormTags = append(gormTags, "autoUpdateTime:nano")
			}
			// Check if a default value is set
			if column.DefaultValue != nil && column.DefaultValue.GetValue() != "" {
				gormTags = append(gormTags, fmt.Sprintf("default:%v", column.DefaultValue.GetValue()))
			}
			// Check if size is specified
			if column.Size != nil {
				gormTags = append(gormTags, fmt.Sprintf("size:%v", column.Size.GetValue()))
			}
			// Check if precision is specified
			if column.Precision != nil {
				gormTags = append(gormTags, fmt.Sprintf("precision:%v", column.Precision.GetValue()))
			}
			// Check if scale is specified
			if column.Scale != nil {
				gormTags = append(gormTags, fmt.Sprintf("scale:%v", column.Scale.GetValue()))
			}
			// Check if the column is nullable
			if column.Required != nil && column.Required.GetValue() {
				gormTags = append(gormTags, "not null")
			}
		}

		tags := strings.Join(gormTags, ";")

		pascalCaseColumnName := utils.SnakeCaseToPascalCase(column.Name)
		switch column.Type {
		case SpannerTableDataType_BOOL.String():
			instance.AddField(pascalCaseColumnName, false, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_INT64.String():
			instance.AddField(pascalCaseColumnName, int64(0), fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_FLOAT64.String():
			instance.AddField(pascalCaseColumnName, float64(0), fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_STRING.String():
			instance.AddField(pascalCaseColumnName, "", fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_BYTES.String():
			instance.AddField(pascalCaseColumnName, []byte{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_DATE.String():
			instance.AddField(pascalCaseColumnName, spanner.NullDate{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_TIMESTAMP.String():
			instance.AddField(pascalCaseColumnName, time.Time{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_JSON.String():
			instance.AddField(pascalCaseColumnName, spanner.NullJSON{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_PROTO.String():
			// TODO: Revisit implementation later
			//msg, err := utils.MessageFromFileDescriptorSet(column.ProtoFileDescriptorSet.ProtoPackage.GetValue(), column.ProtoFileDescriptorSet.fileDescriptorSet)
			//if err != nil {
			//	return nil, status.Errorf(codes.Internal, "Error getting message from file descriptor set: %v", err)
			//}
			//
			instance.AddField(pascalCaseColumnName, customdatatypes.ProtoMessage{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_STRING_ARRAY.String():
			instance.AddField(pascalCaseColumnName, customdatatypes.StringArray{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_INT64_ARRAY.String():
			instance.AddField(pascalCaseColumnName, customdatatypes.Int64Array{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_FLOAT32_ARRAY.String():
			instance.AddField(pascalCaseColumnName, customdatatypes.Float32Array{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_FLOAT64_ARRAY.String():
			instance.AddField(pascalCaseColumnName, customdatatypes.Float64Array{}, fmt.Sprintf("gorm:\"%s\"", tags))
		default:
			return nil, errors.New("unknown column type")
		}

	}

	return instance.Build().New(), nil
}

func fetchDescriptorSet(ctx context.Context, fds *ProtoFileDescriptorSet) ([]byte, error) {
	switch fds.FileDescriptorSetPathSource {
	case ProtoFileDescriptorSetSourceGcs:
		uri := strings.TrimPrefix(fds.FileDescriptorSetPath.GetValue(), "gcs:")
		data, _, err := utils.ReadGcsUri(ctx, uri)
		if err != nil {
			return nil, err
		}

		return data, nil
	case ProtoFileDescriptorSetSourceUrl:
		path := strings.TrimPrefix(fds.FileDescriptorSetPath.GetValue(), "url:")
		data, err := utils.ReadUrl(ctx, path)
		if err != nil {
			return nil, err
		}

		return data, nil
	default:
		return nil, errors.New("unknown source")
	}
}

// CreateProtoBundle creates a proto bundle in a Spanner database.
func CreateProtoBundle(ctx context.Context, databaseName string, protoPackageName string, descriptorSet []byte) error {
	// "projects/my-project/instances/my-instance/database/my-db"
	client, err := spannerAdmin.NewDatabaseAdminClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	// Get database state
	database, err := client.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: databaseName,
	})
	if err != nil {
		return err
	}

	// Unmarshal the proto file descriptor set
	fds := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(descriptorSet, fds); err != nil {
		return status.Errorf(codes.Internal, "Error unmarshalling proto file descriptor set: %v", err)
	}

	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return status.Errorf(codes.Internal, "Error creating proto files: %v", err)
	}

	var getProtoPackageNamesFn func(ctx context.Context, parent string, desc protoreflect.Descriptor) ([]string, error)
	getProtoPackageNamesFn = func(ctx context.Context, parent string, desc protoreflect.Descriptor) ([]string, error) {
		var protoPackageNames []string
		switch d := desc.(type) {
		case protoreflect.MessageDescriptor:
			// Add the proto package name
			protoPackageNames = append(protoPackageNames, fmt.Sprintf("%s", d.FullName()))

			nestedProtoPackageParentNamesMap := map[string]protoreflect.Descriptor{}

			for i := 0; i < d.Fields().Len(); i++ {
				field := d.Fields().Get(i)

				switch field.Kind() {
				case protoreflect.MessageKind:
					// Get nested proto package names
					nestedProtoPackageNames, err := getProtoPackageNamesFn(ctx, parent, field.Message())
					if err != nil {
						return nil, err
					}
					protoPackageNames = append(protoPackageNames, nestedProtoPackageNames...)

					// Get the nested proto package names of the parent
					if field.Message().Parent() != nil {
						nestedProtoPackageParentName := fmt.Sprintf("%s", field.Message().Parent().FullName())
						nestedProtoPackageParentNamesMap[nestedProtoPackageParentName] = field.Message().Parent()
					}
				case protoreflect.EnumKind:
					protoPackageNames = append(protoPackageNames, fmt.Sprintf("%s", field.Enum().FullName()))

					// Get the nested proto package names of the parent
					if field.Enum().Parent() != nil {
						nestedProtoPackageParentName := fmt.Sprintf("%s", field.Enum().Parent().FullName())
						nestedProtoPackageParentNamesMap[nestedProtoPackageParentName] = field.Enum().Parent()
					}
				}
			}

			// Get the nested proto package names of the parents
			for nestedProtoPackageParentName, nestedDesc := range nestedProtoPackageParentNamesMap {
				if nestedProtoPackageParentName == parent {
					continue
				}

				nestedProtoPackageNames, err := getProtoPackageNamesFn(ctx, nestedProtoPackageParentName, nestedDesc)
				if err != nil {
					return nil, err
				}

				protoPackageNames = append(protoPackageNames, nestedProtoPackageNames...)
			}
		case protoreflect.EnumDescriptor:
			// Add the proto package name
			protoPackageNames = append(protoPackageNames, fmt.Sprintf("%s", d.FullName()))

			if d.Parent() != nil {
				// Get the nested proto package names of the parent
				nestedProtoPackageParentName := fmt.Sprintf("%s", d.Parent().FullName())

				// Get the nested proto package names of the parents
				nestedProtoPackageNames, err := getProtoPackageNamesFn(ctx, nestedProtoPackageParentName, d.Parent())
				if err != nil {
					return nil, err
				}

				protoPackageNames = append(protoPackageNames, nestedProtoPackageNames...)

			}
		}

		return protoPackageNames, nil
	}

	// Get the message/enum descriptor
	desc, err := files.FindDescriptorByName(protoreflect.FullName(protoPackageName))
	if err != nil {
		return status.Errorf(codes.Internal, "Error finding descriptor for %s: %v", protoPackageName, err)
	}

	// Get the proto package names including nested messages and enums
	protoPackageNames, err := getProtoPackageNamesFn(ctx, protoPackageName, desc)
	if err != nil {
		return err
	}

	// Remove any duplicates
	protoPackageNames = alUtils.Unique(protoPackageNames)

	// Sort the proto package names
	sort.Strings(protoPackageNames)

	updateDatabaseDdl := func(ctx context.Context, databaseName string, statements []string, descriptorSet []byte) error {
		// Create the proto bundle
		updateDdlOp, err := client.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
			Database:         database.GetName(),
			Statements:       statements,
			ProtoDescriptors: descriptorSet,
		})
		if err != nil {
			return err
		}

		return updateDdlOp.Wait(ctx)
	}

	formattedProtoPackageNames := alUtils.Transform(protoPackageNames, func(name string) string {
		return fmt.Sprintf("`%s`", name)
	})

	createStatement := fmt.Sprintf("CREATE PROTO BUNDLE (%s)", strings.Join(formattedProtoPackageNames, ", "))
	err = updateDatabaseDdl(ctx, databaseName, []string{createStatement}, descriptorSet)
	if err != nil {
		if status.Code(err) != codes.AlreadyExists && status.Code(err) != codes.InvalidArgument {
			return err
		}

		// Try to insert the proto bundle
		insertStatement := fmt.Sprintf("ALTER PROTO BUNDLE INSERT (%s)", strings.Join(formattedProtoPackageNames, ", "))
		err = updateDatabaseDdl(ctx, databaseName, []string{insertStatement}, descriptorSet)
		if err != nil {
			if status.Code(err) != codes.AlreadyExists && status.Code(err) != codes.InvalidArgument {
				return err
			}

			// Try to update the proto bundle
			updateStatement := fmt.Sprintf("ALTER PROTO BUNDLE UPDATE (%s)", strings.Join(formattedProtoPackageNames, ", "))
			err = updateDatabaseDdl(ctx, databaseName, []string{updateStatement}, descriptorSet)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
