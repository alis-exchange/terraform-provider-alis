package schema

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	spannerAdmin "cloud.google.com/go/spanner/admin/database/apiv1"
	spannergorm "github.com/googleapis/go-gorm-spanner"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gorm.io/gorm"
)

var (
	// TestProject is the project used for testing.
	testProject string
	// TestInstance is the instance used for testing.
	testInstance string
)

func init() {

	testProject = os.Getenv("ALIS_OS_PROJECT")
	testInstance = os.Getenv("ALIS_OS_INSTANCE")
	if testProject == "" {
		log.Fatalf("ALIS_OS_PROJECT must be set for integration tests")
	}
	if testInstance == "" {
		log.Fatalf("ALIS_OS_INSTANCE must be set for integration tests")
	}
}

func Test_SpannerTable_createDdl(t1 *testing.T) {
	type fields struct {
		Name       string
		Schema     *SpannerTableSchema
		Interleave *SpannerTableInterleave
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "SpannerTable.createDdl",
			fields: fields{
				Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", testProject, testInstance, "play-np", "tf_test"),
				Schema: &SpannerTableSchema{
					Columns: []*SpannerTableColumn{
						{
							Name:           "id",
							IsPrimaryKey:   wrapperspb.Bool(true),
							IsComputed:     wrapperspb.Bool(false),
							ComputationDdl: nil,

							AutoCreateTime:         wrapperspb.Bool(true),
							AutoUpdateTime:         wrapperspb.Bool(true),
							Type:                   SpannerTableDataTypeInt64.String(),
							Size:                   wrapperspb.Int64(255),
							Required:               wrapperspb.Bool(true),
							DefaultValue:           wrapperspb.String("(GET_NEXT_SEQUENCE_VALUE(SEQUENCE MySequence))"),
							ProtoFileDescriptorSet: nil,
						},
						{
							Name:                   "name",
							IsPrimaryKey:           wrapperspb.Bool(true),
							IsComputed:             wrapperspb.Bool(false),
							ComputationDdl:         nil,
							Type:                   SpannerTableDataTypeString.String(),
							Size:                   wrapperspb.Int64(255),
							Required:               wrapperspb.Bool(true),
							DefaultValue:           nil,
							ProtoFileDescriptorSet: nil,
						},
						{
							Name:           "proto",
							IsPrimaryKey:   wrapperspb.Bool(false),
							IsComputed:     wrapperspb.Bool(false),
							ComputationDdl: nil,
							Type:           SpannerTableDataTypeProto.String(),
							ProtoFileDescriptorSet: &ProtoFileDescriptorSet{
								ProtoPackage: wrapperspb.String(`play.nn.app.v1.App`),
							},
						},
						{
							Name:           "update_time",
							Type:           SpannerTableDataTypeTimestamp.String(),
							AutoUpdateTime: wrapperspb.Bool(true),
						},
					},
				},
				Interleave: &SpannerTableInterleave{
					ParentTable: "parent_table",
					OnDelete:    SpannerTableConstraintActionCascade,
				},
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &SpannerTable{
				Name:       tt.fields.Name,
				Schema:     tt.fields.Schema,
				Interleave: tt.fields.Interleave,
			}

			got, err := t.createDdl()
			if (err != nil) != tt.wantErr {
				t1.Errorf("createDdl() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t1.Errorf("createDdl() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerTable_alterDdl(t1 *testing.T) {
	type fields struct {
		Name       string
		Schema     *SpannerTableSchema
		Interleave *SpannerTableInterleave
	}
	type args struct {
		existingTable *SpannerTable
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "SpannerTable.alterDdl",
			fields: fields{
				Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", testProject, testInstance, "play-np", "tf_test"),
				Schema: &SpannerTableSchema{
					Columns: []*SpannerTableColumn{
						{
							Name:         "id",
							IsPrimaryKey: wrapperspb.Bool(true),

							Type:     "INT64",
							Size:     wrapperspb.Int64(255),
							Required: wrapperspb.Bool(true),
						},
						{
							Name:         "display_name",
							IsPrimaryKey: wrapperspb.Bool(false),
							Required:     wrapperspb.Bool(true),
							Type:         "STRING",
							Size:         wrapperspb.Int64(250),
							DefaultValue: wrapperspb.String("10.0"),
						},
						{
							Name:         "is_active",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "BOOL",
						},
						{
							Name:         "latest_return",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type:         "FLOAT64",
							DefaultValue: wrapperspb.String("10.0"),
						},
						{
							Name:         "inception_date",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "DATE",
						},
						{
							Name:         "last_refreshed_at",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "TIMESTAMP",
						},
						{
							Name:         "metadata",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "JSON",
						},
						{
							Name:         "data",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "BYTES",
						},
						{
							Name:         "user",
							IsPrimaryKey: wrapperspb.Bool(false),
							Required:     wrapperspb.Bool(true),
							Type:         "PROTO",
							ProtoFileDescriptorSet: &ProtoFileDescriptorSet{
								ProtoPackage: wrapperspb.String("alis.open.iam.v1.User"),
							},
						},
						{
							Name:           "user_name",
							IsPrimaryKey:   wrapperspb.Bool(false),
							IsComputed:     wrapperspb.Bool(true),
							ComputationDdl: wrapperspb.String("user.name"),
							Type:           "STRING",
							Size:           wrapperspb.Int64(255),
						},
						//{
						//	Name:         "tags",
						//	IsPrimaryKey: wrapperspb.Bool(false),
						//
						//	Type: "ARRAY<STRING>",
						//},
						{
							Name:         "ids",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "ARRAY<INT64>",
						},
						{
							Name:         "prices",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "ARRAY<FLOAT64>",
						},
						{
							Name:         "discounts",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "ARRAY<FLOAT32>",
						},
					},
				},
				//Interleave: &SpannerTableInterleave{},
			},
			args: args{
				existingTable: &SpannerTable{
					Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", testProject, testInstance, "play-np", "tf_test"),
					Schema: &SpannerTableSchema{
						Columns: []*SpannerTableColumn{
							{
								Name:         "id",
								IsPrimaryKey: wrapperspb.Bool(true),

								Type:     "INT64",
								Size:     wrapperspb.Int64(255),
								Required: wrapperspb.Bool(true),
							},
							{
								Name:         "display_name",
								IsPrimaryKey: wrapperspb.Bool(false),

								Type: "STRING",
								Size: wrapperspb.Int64(255),
							},
							{
								Name:         "is_active",
								IsPrimaryKey: wrapperspb.Bool(false),

								Type: "BOOL",
							},
							{
								Name:         "latest_return",
								IsPrimaryKey: wrapperspb.Bool(false),

								Type:         "FLOAT64",
								DefaultValue: wrapperspb.String("0.0"),
							},
							{
								Name:         "inception_date",
								IsPrimaryKey: wrapperspb.Bool(false),

								Type: "DATE",
							},
							{
								Name:         "last_refreshed_at",
								IsPrimaryKey: wrapperspb.Bool(false),

								Type: "TIMESTAMP",
							},
							{
								Name:         "metadata",
								IsPrimaryKey: wrapperspb.Bool(false),

								Type: "JSON",
							},
							{
								Name:         "data",
								IsPrimaryKey: wrapperspb.Bool(false),
								Required:     wrapperspb.Bool(true),
								Type:         "BYTES",
							},
							{
								Name:         "user",
								IsPrimaryKey: wrapperspb.Bool(false),

								Type: "PROTO",
								ProtoFileDescriptorSet: &ProtoFileDescriptorSet{
									ProtoPackage: wrapperspb.String("alis.open.iam.v1.User"),
								},
							},
							{
								Name:         "user_name",
								IsPrimaryKey: wrapperspb.Bool(false),

								IsComputed:     wrapperspb.Bool(true),
								ComputationDdl: wrapperspb.String("user.name"),
								Type:           "STRING",
							},
							{
								Name:         "tags",
								IsPrimaryKey: wrapperspb.Bool(false),

								Type: "ARRAY<STRING>",
							},
							{
								Name:         "ids",
								IsPrimaryKey: wrapperspb.Bool(false),

								Type: "ARRAY<INT64>",
							},
							{
								Name:         "prices",
								IsPrimaryKey: wrapperspb.Bool(false),

								Type: "ARRAY<FLOAT64>",
							},
							//{
							//	Name:         "discounts",
							//	IsPrimaryKey: wrapperspb.Bool(false),
							//
							//	Type: "ARRAY<FLOAT32>",
							//},
						},
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &SpannerTable{
				Name:       tt.fields.Name,
				Schema:     tt.fields.Schema,
				Interleave: tt.fields.Interleave,
			}
			got, _, err := t.alterDdl(tt.args.existingTable)
			if (err != nil) != tt.wantErr {
				t1.Errorf("alterDdl() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t1.Errorf("alterDdl() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerTable_Create(t1 *testing.T) {
	type fields struct {
		Name       string
		Schema     *SpannerTableSchema
		Interleave *SpannerTableInterleave
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *SpannerTable
		wantErr bool
	}{
		{
			name: "SpannerTable.Create",
			fields: fields{
				Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", testProject, testInstance, "play-np", "tf_test"),
				Schema: &SpannerTableSchema{
					Columns: []*SpannerTableColumn{
						{
							Name:         "id",
							IsPrimaryKey: wrapperspb.Bool(true),

							Type:     "INT64",
							Size:     wrapperspb.Int64(255),
							Required: wrapperspb.Bool(true),
						},
						{
							Name:         "display_name",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "STRING",
							Size: wrapperspb.Int64(255),
						},
						{
							Name:         "is_active",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "BOOL",
						},
						{
							Name:         "latest_return",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type:         "FLOAT64",
							DefaultValue: wrapperspb.String("0.0"),
						},
						{
							Name:         "inception_date",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "DATE",
						},
						{
							Name:         "last_refreshed_at",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "TIMESTAMP",
						},
						{
							Name:         "metadata",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "JSON",
						},
						{
							Name:         "data",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "BYTES",
						},
						{
							Name:         "user",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "PROTO",
							ProtoFileDescriptorSet: &ProtoFileDescriptorSet{
								ProtoPackage: wrapperspb.String("alis.open.iam.v1.User"),
							},
						},
						{
							Name:         "user_name",
							IsPrimaryKey: wrapperspb.Bool(false),

							IsComputed:     wrapperspb.Bool(true),
							ComputationDdl: wrapperspb.String("user.name"),
							Type:           "STRING",
						},
						{
							Name:         "tags",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "ARRAY<STRING>",
						},
						{
							Name:         "ids",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "ARRAY<INT64>",
						},
						{
							Name:         "prices",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "ARRAY<FLOAT64>",
						},
						{
							Name:         "discounts",
							IsPrimaryKey: wrapperspb.Bool(false),

							Type: "ARRAY<FLOAT32>",
						},
					},
				},
				//Interleave: &SpannerTableInterleave{},
			},
			args: args{
				ctx: context.Background(),
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &SpannerTable{
				Name:       tt.fields.Name,
				Schema:     tt.fields.Schema,
				Interleave: tt.fields.Interleave,
			}
			got, err := t.Create(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t1.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t1.Errorf("Create() got = %v, want %v", got, tt.want)
			}

		})
	}
}

func TestSpannerTable_Get(t1 *testing.T) {
	type fields struct {
		Name       string
		Schema     *SpannerTableSchema
		Interleave *SpannerTableInterleave
	}
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *SpannerTable
		wantErr bool
	}{
		{
			name: "SpannerTable.Get",
			fields: fields{
				Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", testProject, testInstance, "play-np", "tf_test"),
			},
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", testProject, testInstance, "play-np", "tf_test"),
			},
			want:    &SpannerTable{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &SpannerTable{
				Name:       tt.fields.Name,
				Schema:     tt.fields.Schema,
				Interleave: tt.fields.Interleave,
			}

			got, err := t.Get(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t1.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t1.Errorf("Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerTable_Delete(t1 *testing.T) {
	type fields struct {
		Name       string
		Schema     *SpannerTableSchema
		Interleave *SpannerTableInterleave
	}
	type args struct {
		ctx         context.Context
		adminClient *spannerAdmin.DatabaseAdminClient
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "SpannerTable.Delete",
			fields: fields{
				Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", testProject, testInstance, "play-np", "tf_test"),
				Schema: &SpannerTableSchema{
					Columns: []*SpannerTableColumn{},
				},
			},
			args: args{
				ctx: context.Background(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &SpannerTable{
				Name:       tt.fields.Name,
				Schema:     tt.fields.Schema,
				Interleave: tt.fields.Interleave,
			}
			err := t.Delete(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t1.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}

			db, err := gorm.Open(
				spannergorm.New(
					spannergorm.Config{
						DriverName: "spanner",
						DSN:        t.GetDatabase(),
					},
				),
				&gorm.Config{
					PrepareStmt: true,
				},
			)
			if err != nil {
				t1.Errorf("gorm.Open() error = %v", err)
				return
			}

			// Delete column metadata
			if err := DeleteColumnMetadata(db, t.GetTableId(), []*SpannerTableColumn{}); err != nil {
				t1.Errorf("DeleteColumnMetadata() error = %v", err)
			}
		})
	}
}

func Test_parseSpannerType(t *testing.T) {
	type args struct {
		columnType string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "parseSpannerType.EMPTY",
			args: args{
				columnType: "",
			},
			want: "",
		},
		{
			name: "parseSpannerType.STRING(MAX)",
			args: args{
				columnType: "STRING(MAX)",
			},
			want: "STRING",
		},
		{
			name: "parseSpannerType.STRING(255)",
			args: args{
				columnType: "STRING(255)",
			},
			want: "STRING",
		},
		{
			name: "parseSpannerType.BYTES(MAX)",
			args: args{
				columnType: "BYTES(MAX)",
			},
			want: "BYTES",
		},
		{
			name: "parseSpannerType.BYTES(255)",
			args: args{
				columnType: "BYTES(255)",
			},
			want: "BYTES",
		},
		{
			name: "parseSpannerType.ARRAY<STRING(MAX)>",
			args: args{
				columnType: "ARRAY<STRING(MAX)>",
			},
			want: "ARRAY<STRING>",
		},
		{
			name: "parseSpannerType.ARRAY<STRING(255)>",
			args: args{
				columnType: "ARRAY<STRING(255)>",
			},
			want: "ARRAY<STRING>",
		},
		{
			name: "parseSpannerType.ARRAY<INT64>",
			args: args{
				columnType: "ARRAY<INT64>",
			},
			want: "ARRAY<INT64>",
		},
		{
			name: "parseSpannerType.ARRAY<FLOAT32>",
			args: args{
				columnType: "ARRAY<FLOAT32>",
			},
			want: "ARRAY<FLOAT32>",
		},
		{
			name: "parseSpannerType.ARRAY<FLOAT64>",
			args: args{
				columnType: "ARRAY<FLOAT64>",
			},
			want: "ARRAY<FLOAT64>",
		},
		{
			name: "parseSpannerType.PROTO",
			args: args{
				columnType: "PROTO<my.example.package.Message>",
			},
			want: "PROTO",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSpannerType(tt.args.columnType); got != tt.want {
				t.Errorf("parseSpannerType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseSpannerSize(t *testing.T) {
	type args struct {
		columnType string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "parseSpannerSize.EMPTY",
			args: args{
				columnType: "",
			},
			want: "",
		},
		{
			name: "parseSpannerSize.STRING(MAX)",
			args: args{
				columnType: "STRING(MAX)",
			},
			want: "MAX",
		},
		{
			name: "parseSpannerSize.STRING(255)",
			args: args{
				columnType: "STRING(255)",
			},
			want: "255",
		},
		{
			name: "parseSpannerSize.BYTES(MAX)",
			args: args{
				columnType: "BYTES(MAX)",
			},
			want: "MAX",
		},
		{
			name: "parseSpannerSize.BYTES(255)",
			args: args{
				columnType: "BYTES(255)",
			},
			want: "255",
		},
		{
			name: "parseSpannerSize.ARRAY<STRING(MAX)>",
			args: args{
				columnType: "ARRAY<STRING(MAX)>",
			},
			want: "MAX",
		},
		{
			name: "parseSpannerSize.ARRAY<STRING(255)>",
			args: args{
				columnType: "ARRAY<STRING(255)>",
			},
			want: "255",
		},
		{
			name: "parseSpannerSize.ARRAY<INT64>",
			args: args{
				columnType: "ARRAY<INT64>",
			},
			want: "",
		},
		{
			name: "parseSpannerSize.ARRAY<FLOAT32>",
			args: args{
				columnType: "ARRAY<FLOAT32>",
			},
			want: "",
		},
		{
			name: "parseSpannerSize.ARRAY<FLOAT64>",
			args: args{
				columnType: "ARRAY<FLOAT64>",
			},
			want: "",
		},
		{
			name: "parseSpannerSize.PROTO",
			args: args{
				columnType: "PROTO<my.example.package.Message>",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSpannerSize(tt.args.columnType); got != tt.want {
				t.Errorf("parseSpannerSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseSpannerProtoPackage(t *testing.T) {
	type args struct {
		columnType string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "parseSpannerProtoPackage.EMPTY",
			args: args{
				columnType: "",
			},
			want: "",
		},
		{
			name: "parseSpannerProtoPackage.STRING(MAX)",
			args: args{
				columnType: "STRING(MAX)",
			},
			want: "",
		},
		{
			name: "parseSpannerProtoPackage.STRING(255)",
			args: args{
				columnType: "STRING(255)",
			},
			want: "",
		},
		{
			name: "parseSpannerProtoPackage.BYTES(MAX)",
			args: args{
				columnType: "BYTES(MAX)",
			},
			want: "",
		},
		{
			name: "parseSpannerProtoPackage.BYTES(255)",
			args: args{
				columnType: "BYTES(255)",
			},
			want: "",
		},
		{
			name: "parseSpannerProtoPackage.ARRAY<STRING(MAX)>",
			args: args{
				columnType: "ARRAY<STRING(MAX)>",
			},
			want: "",
		},
		{
			name: "parseSpannerProtoPackage.ARRAY<STRING(255)>",
			args: args{
				columnType: "ARRAY<STRING(255)>",
			},
			want: "",
		},
		{
			name: "parseSpannerProtoPackage.ARRAY<INT64>",
			args: args{
				columnType: "ARRAY<INT64>",
			},
			want: "",
		},
		{
			name: "parseSpannerProtoPackage.ARRAY<FLOAT32>",
			args: args{
				columnType: "ARRAY<FLOAT32>",
			},
			want: "",
		},
		{
			name: "parseSpannerProtoPackage.ARRAY<FLOAT64>",
			args: args{
				columnType: "ARRAY<FLOAT64>",
			},
			want: "",
		},
		{
			name: "parseSpannerProtoPackage.PROTO",
			args: args{
				columnType: "PROTO<my.example.package.Message>",
			},
			want: "my.example.package.Message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSpannerProtoPackage(tt.args.columnType); got != tt.want {
				t.Errorf("parseSpannerProtoPackage() = %v, want %v", got, tt.want)
			}
		})
	}
}
