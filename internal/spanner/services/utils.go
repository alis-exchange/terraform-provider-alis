package services

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	spanner "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	_ "github.com/googleapis/go-sql-spanner"
	dynamicstruct "github.com/ompluscator/dynamic-struct"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"terraform-provider-alis/internal/utils"
)

// SpannerTableDataType is a type for Spanner table column data types.
type SpannerTableDataType int64

const (
	SpannerTableDataType_BOOL SpannerTableDataType = iota + 1
	SpannerTableDataType_INT64
	SpannerTableDataType_FLOAT64
	SpannerTableDataType_STRING
	SpannerTableDataType_BYTES
	SpannerTableDataType_DATE
	SpannerTableDataType_TIMESTAMP
	SpannerTableDataType_JSON
	SpannerTableDataType_PROTO
)

func (t SpannerTableDataType) String() string {
	return [...]string{"BOOL", "INT64", "FLOAT64", "STRING", "BYTES", "DATE", "TIMESTAMP", "JSON", "PROTO"}[t-1]
}

// SpannerTableDataTypes is a list of all Spanner table column data types.
var SpannerTableDataTypes = []string{
	SpannerTableDataType_BOOL.String(),
	SpannerTableDataType_INT64.String(),
	SpannerTableDataType_FLOAT64.String(),
	SpannerTableDataType_STRING.String(),
	SpannerTableDataType_BYTES.String(),
	SpannerTableDataType_DATE.String(),
	SpannerTableDataType_TIMESTAMP.String(),
	SpannerTableDataType_JSON.String(),
	SpannerTableDataType_PROTO.String(),
}

type SpannerTableIndexColumnOrder int64

const (
	SpannerTableIndexColumnOrder_UNSPECIFIED SpannerTableIndexColumnOrder = iota
	SpannerTableIndexColumnOrder_ASC
	SpannerTableIndexColumnOrder_DESC
)

type ProtoFileDescriptorSetSource int64

const (
	ProtoFileDescriptorSetSourceUNSPECIFIED ProtoFileDescriptorSetSource = iota
	ProtoFileDescriptorSetSourceGcs
	ProtoFileDescriptorSetSourceUrl
)

func (s SpannerTableIndexColumnOrder) String() string {
	return [...]string{"unspecified", "asc", "desc"}[s]
}

var SpannerTableIndexColumnOrders = []string{
	SpannerTableIndexColumnOrder_ASC.String(),
	SpannerTableIndexColumnOrder_DESC.String(),
}

type ColumnMetadataMeta struct {
	Type                        string `json:"type"`
	Size                        string `json:"size"`
	Precision                   string `json:"precision"`
	Scale                       string `json:"scale"`
	Required                    string `json:"required"`
	AutoIncrement               string `json:"auto_increment"`
	Unique                      string `json:"unique"`
	DefaultValue                string `json:"default_value"`
	IsPrimaryKey                string `json:"is_primary_key"`
	ProtoPackage                string `json:"proto_package"`
	FileDescriptorSetPath       string `json:"file_descriptor_set_path"`
	FileDescriptorSetPathSource string `json:"file_descriptor_set_path_source"`
}

// Value returns value of CustomerInfo struct and implements driver.Valuer interface
func (c *ColumnMetadataMeta) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan scans value into Jsonb and implements sql.Scanner interface
func (c *ColumnMetadataMeta) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &c)
}

type ColumnMetadata struct {
	TableName  string `gorm:"primaryKey"`
	ColumnName string `gorm:"primaryKey"`
	Metadata   *ColumnMetadataMeta
	CreatedAt  time.Time // Automatically managed by GORM for creation time
	UpdatedAt  time.Time // Automatically managed by GORM for update time
}

type Index struct {
	IndexName       string
	IndexType       string
	ColumnName      string
	ColumnOrdering  string
	IsUnique        bool
	OrdinalPosition int
}

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
	if err := db.AutoMigrate(&ColumnMetadata{}); err != nil {
		return err
	}

	// Update ColumnMetadata table
	for _, column := range columns {

		meta := &ColumnMetadataMeta{}
		//meta["type"] = column.Type
		//if column.Size != nil {
		//	meta["size"] = fmt.Sprintf("%d", column.Size.GetValue())
		//} else {
		//	meta["size"] = "nil"
		//}
		//if column.Precision != nil {
		//	meta["precision"] = fmt.Sprintf("%d", column.Precision.GetValue())
		//} else {
		//	meta["precision"] = "nil"
		//}
		//if column.Scale != nil {
		//	meta["scale"] = fmt.Sprintf("%d", column.Scale.GetValue())
		//} else {
		//	meta["scale"] = "nil"
		//}
		//if column.Required != nil {
		//	meta["required"] = fmt.Sprintf("%t", column.Required.GetValue())
		//} else {
		//	meta["required"] = "nil"
		//}
		//if column.AutoIncrement != nil {
		//	meta["auto_increment"] = fmt.Sprintf("%t", column.AutoIncrement.GetValue())
		//} else {
		//	meta["auto_increment"] = "nil"
		//}
		//if column.Unique != nil {
		//	meta["unique"] = fmt.Sprintf("%t", column.Unique.GetValue())
		//} else {
		//	meta["unique"] = "nil"
		//}
		//if column.DefaultValue != nil {
		//	meta["default_value"] = column.DefaultValue.GetValue()
		//} else {
		//	meta["default_value"] = "nil"
		//}
		//if column.IsPrimaryKey != nil {
		//	meta["is_primary_key"] = fmt.Sprintf("%t", column.IsPrimaryKey.GetValue())
		//} else {
		//	meta["is_primary_key"] = "nil"
		//}
		//if column.ProtoFileDescriptorSet != nil {
		//	if column.ProtoFileDescriptorSet.ProtoPackage != nil {
		//		meta["proto_package"] = column.ProtoFileDescriptorSet.ProtoPackage.GetValue()
		//	} else {
		//		meta["proto_package"] = "nil"
		//	}
		//	if column.ProtoFileDescriptorSet.FileDescriptorSetPath != nil {
		//		meta["file_descriptor_set_path"] = column.ProtoFileDescriptorSet.FileDescriptorSetPath.GetValue()
		//	} else {
		//		meta["file_descriptor_set_path"] = "nil"
		//	}
		//	if column.ProtoFileDescriptorSet.FileDescriptorSetPathSource != ProtoFileDescriptorSetSourceUNSPECIFIED {
		//		meta["file_descriptor_set_path_source"] = fmt.Sprintf("%d", column.ProtoFileDescriptorSet.FileDescriptorSetPathSource)
		//	} else {
		//		meta["file_descriptor_set_path_source"] = "nil"
		//	}
		//}
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

	type ColumnIndex struct {
		Name     string
		Unique   bool
		Priority int
		Order    string
	}

	type ColumnIndices struct {
		Indices []*ColumnIndex
	}

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
			instance.AddField(pascalCaseColumnName, datatypes.Date{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_TIMESTAMP.String():
			instance.AddField(pascalCaseColumnName, time.Time{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_JSON.String():
			instance.AddField(pascalCaseColumnName, datatypes.JSON{}, fmt.Sprintf("gorm:\"%s\"", tags))
		case SpannerTableDataType_PROTO.String():
			// TODO: Implement this when support for proto is added
			//	msg, err := utils.MessageFromFileDescriptorSet(column.ProtoFileDescriptorSet.ProtoPackage.GetValue(), column.ProtoFileDescriptorSet.fileDescriptorSet)
			//	if err != nil {
			//		return nil, status.Errorf(codes.Internal, "Error getting message from file descriptor set: %v", err)
			//	}
			//
			//	instance.AddField(pascalCaseColumnName, msg, fmt.Sprintf("gorm:\"%s\"", tags))
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
	client, err := spanner.NewDatabaseAdminClient(ctx)
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

	desc, err := files.FindDescriptorByName(protoreflect.FullName(protoPackageName))
	if err != nil {
		return status.Errorf(codes.Internal, "Error finding descriptor for %s: %v", protoPackageName, err)
	}

	// TODO: Revisit later
	switch d := desc.(type) {
	case protoreflect.MessageDescriptor:
		_ = d
		//enums := d.Enums()
		//for i := 0; i < enums.Len(); i++ {
		//	if err := CreateProtoBundle(ctx, databaseName, fmt.Sprintf("%s", enums.Get(i).FullName()), descriptorSet); err != nil {
		//		return err
		//	}
		//}

		//messages := d.Messages()
		//for i := 0; i < messages.Len(); i++ {
		//
		//}
	}

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

	createStatement := fmt.Sprintf("CREATE PROTO BUNDLE (`%s`)", protoPackageName)
	err = updateDatabaseDdl(ctx, databaseName, []string{createStatement}, descriptorSet)
	if err != nil {
		if status.Code(err) != codes.AlreadyExists && status.Code(err) != codes.InvalidArgument {
			return err
		}

		// Try to insert the proto bundle
		insertStatement := fmt.Sprintf("ALTER PROTO BUNDLE INSERT (%s)", protoPackageName)
		err = updateDatabaseDdl(ctx, databaseName, []string{insertStatement}, descriptorSet)
		if err != nil {
			if status.Code(err) != codes.AlreadyExists && status.Code(err) != codes.InvalidArgument {
				return err
			}

			// Try to update the proto bundle
			updateStatement := fmt.Sprintf("ALTER PROTO BUNDLE UPDATE (`%s`)", protoPackageName)
			err = updateDatabaseDdl(ctx, databaseName, []string{updateStatement}, descriptorSet)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
