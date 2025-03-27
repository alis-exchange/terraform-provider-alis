package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	spannerAdmin "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	_ "github.com/googleapis/go-sql-spanner"
	alUtils "go.alis.build/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gorm.io/gorm"
	"terraform-provider-alis/internal/spanner/schema"
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

func fetchDescriptorSet(ctx context.Context, fds *schema.ProtoFileDescriptorSet) ([]byte, error) {
	switch fds.FileDescriptorSetPathSource {
	case schema.ProtoFileDescriptorSetSourceGcs:
		uri := strings.TrimPrefix(fds.FileDescriptorSetPath.GetValue(), "gcs:")
		data, _, err := utils.ReadGcsUri(ctx, uri)
		if err != nil {
			return nil, err
		}

		return data, nil
	case schema.ProtoFileDescriptorSetSourceUrl:
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
