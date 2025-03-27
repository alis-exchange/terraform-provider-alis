package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-alis/internal/spanner/schema"
)

var (
	// TestProject is the project used for testing.
	TestProject string
	// TestInstance is the instance used for testing.
	TestInstance string
	service      *SpannerService
)

func init() {
	TestProject = os.Getenv("ALIS_OS_PROJECT")
	TestInstance = os.Getenv("ALIS_OS_INSTANCE")

	if TestProject == "" {
		log.Fatalf("ALIS_OS_PROJECT must be set for integration tests")
	}

	if TestInstance == "" {
		log.Fatalf("ALIS_OS_INSTANCE must be set for integration tests")
	}

	service = NewSpannerService(nil)
}

func TestSpannerService_CreateDatabaseRole(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx    context.Context
		parent string
		roleId string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *databasepb.DatabaseRole
		wantErr bool
	}{
		{
			name: "Test_CreateDatabaseRole",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "tf-test"),
				roleId: "admin",
			},
			want: &databasepb.DatabaseRole{
				Name: "admin",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerService{
				GoogleCredentials: tt.fields.GoogleCredentials,
			}
			got, err := s.CreateDatabaseRole(tt.args.ctx, tt.args.parent, tt.args.roleId)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDatabaseRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateDatabaseRole() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestSpannerService_GetDatabaseRole(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *databasepb.DatabaseRole
		wantErr bool
	}{
		{
			name: "Test_GetDatabaseRole",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/databaseRoles/%s", TestProject, TestInstance, "tf-test", "admin"),
			},
			want: &databasepb.DatabaseRole{
				Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/databaseRoles/%s", TestProject, TestInstance, "tf-test", "admin"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerService{
				GoogleCredentials: tt.fields.GoogleCredentials,
			}
			got, err := s.GetDatabaseRole(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDatabaseRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDatabaseRole() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateSpannerTable(t *testing.T) {
	type args struct {
		ctx     context.Context
		parent  string
		tableId string
		table   *schema.SpannerTable
	}
	tests := []struct {
		name    string
		args    args
		want    *schema.SpannerTable
		wantErr bool
	}{
		{
			name: "Test_CreateSpannerTable",
			args: args{
				ctx:     context.Background(),
				parent:  fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "alis-px"),
				tableId: "tf_test",
				table: &schema.SpannerTable{
					Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "alis-px", "tf_test"),
					Schema: &schema.SpannerTableSchema{
						Columns: []*schema.SpannerTableColumn{
							{
								Name:         "key",
								IsPrimaryKey: wrapperspb.Bool(true),
								Type:         "INT64",
								Size:         wrapperspb.Int64(255),
								Required:     wrapperspb.Bool(true),
							},
							{
								Name:     "created_at",
								Type:     "TIMESTAMP",
								Required: wrapperspb.Bool(false),
							},
							{
								Name:           "updated_at",
								Type:           "TIMESTAMP",
								Required:       wrapperspb.Bool(true),
								AutoUpdateTime: wrapperspb.Bool(true),
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.CreateSpannerTable(tt.args.ctx, tt.args.parent, tt.args.tableId, tt.args.table)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSpannerTable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateSpannerTable() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestGetSpannerTable(t *testing.T) {
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    *schema.SpannerTable
		wantErr bool
	}{
		{
			name: "Test_GetSpannerTable",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "mentenova-co", "mentenova_co_dev_62g_Maps"),
			},
			want:    &schema.SpannerTable{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.GetSpannerTable(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpannerTable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSpannerTable() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestUpdateSpannerTable(t *testing.T) {
	type args struct {
		ctx          context.Context
		table        *schema.SpannerTable
		updateMask   *fieldmaskpb.FieldMask
		allowMissing bool
	}
	tests := []struct {
		name    string
		args    args
		want    *schema.SpannerTable
		wantErr bool
	}{
		{
			name: "Test_UpdateSpannerTable",
			args: args{
				ctx: context.Background(),
				table: &schema.SpannerTable{
					Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "play-np", "tf_test"),
					Schema: &schema.SpannerTableSchema{

						Columns: []*schema.SpannerTableColumn{
							{
								Name:         "id",
								IsPrimaryKey: wrapperspb.Bool(true),
								Type:         "INT64",
								Required:     wrapperspb.Bool(true),
							},
							{
								Name: "display_name",
								Type: "STRING",
								Size: wrapperspb.Int64(200),
							},
							{
								Name: "is_active",
								Type: "BOOL",
							},
							{
								Name:         "latest_return",
								Type:         "FLOAT64",
								DefaultValue: wrapperspb.String("10.0"),
							},
							{
								Name: "inception_date",
								Type: "DATE",
							},
							{
								Name: "last_refreshed_at",
								Type: "TIMESTAMP",
							},
							{
								Name: "metadata",
								Type: "JSON",
							},
							{
								Name: "data",
								Type: "BYTES",
							},
							{
								Name: "User",
								Type: "PROTO",
								ProtoFileDescriptorSet: &schema.ProtoFileDescriptorSet{
									ProtoPackage: wrapperspb.String("alis.open.iam.v1.User"),
								},
							},
							{
								Name:           "user_name",
								IsComputed:     wrapperspb.Bool(true),
								ComputationDdl: wrapperspb.String("User.name"),
								Type:           "STRING",
							},
							{
								Name: "tags",
								Type: "ARRAY<STRING>",
								Size: wrapperspb.Int64(255),
							},
							{
								Name: "ids",
								Type: "ARRAY<INT64>",
							},
							{
								Name: "prices",
								Type: "ARRAY<FLOAT64>",
							},
							{
								Name: "discounts",
								Type: "ARRAY<FLOAT32>",
							},
						},
					},
				},
				updateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"schema.columns"},
				},
				allowMissing: false,
			},
			want:    &schema.SpannerTable{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.UpdateSpannerTable(tt.args.ctx, tt.args.table, tt.args.updateMask, tt.args.allowMissing)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateSpannerTable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateSpannerTable() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestDeleteSpannerTable(t *testing.T) {
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    *emptypb.Empty
		wantErr bool
	}{
		{
			name: "Test_DeleteSpannerTable",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "alis-px", "tf_test"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.DeleteSpannerTable(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteSpannerTable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteSpannerTable() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerService_CreateSpannerTableIndex(t *testing.T) {
	type fields struct {
	}
	type args struct {
		ctx    context.Context
		parent string
		index  *SpannerTableIndex
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *SpannerTableIndex
		wantErr bool
	}{
		{
			name: "Test_CreateSpannerTableIndex",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "tf-test", "tftest"),
				index: &SpannerTableIndex{
					Name: "test_idx",
					Columns: []*SpannerTableIndexColumn{
						{
							Name: "name",
						},
						{
							Name:  "display_name",
							Order: SpannerTableIndexColumnOrder_DESC,
						},
						{
							Name:  "state",
							Order: SpannerTableIndexColumnOrder_ASC,
						},
					},
					Unique: &wrapperspb.BoolValue{
						Value: true,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.CreateSpannerTableIndex(tt.args.ctx, tt.args.parent, tt.args.index)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSpannerTableIndex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateSpannerTableIndex() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerService_GetSpannerTableIndex(t *testing.T) {
	type fields struct {
	}
	type args struct {
		ctx    context.Context
		parent string
		name   string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *SpannerTableIndex
		wantErr bool
	}{
		{
			name: "Test_GetSpannerTableIndex",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "tf-test", "tftest"),
				name:   "test_idx",
			},
			want:    &SpannerTableIndex{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := service.GetSpannerTableIndex(tt.args.ctx, tt.args.parent, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpannerTableIndex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSpannerTableIndex() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerService_ListSpannerTableIndices(t *testing.T) {
	type fields struct {
	}
	type args struct {
		ctx    context.Context
		parent string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*SpannerTableIndex
		wantErr bool
	}{
		{
			name: "Test_ListSpannerTableIndices",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "tf-test", "tftest"),
			},
			want:    []*SpannerTableIndex{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := service.ListSpannerTableIndices(tt.args.ctx, tt.args.parent)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListSpannerTableIndices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListSpannerTableIndices() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerService_DeleteIndex(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx       context.Context
		parent    string
		indexName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *emptypb.Empty
		wantErr bool
	}{
		{
			name: "Test_DeleteSpannerTableIndex",
			args: args{
				ctx:       context.Background(),
				parent:    fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "tf-test", "tftest"),
				indexName: "test_idx",
			},
			want:    &emptypb.Empty{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerService{
				GoogleCredentials: tt.fields.GoogleCredentials,
			}
			got, err := s.DeleteSpannerTableIndex(tt.args.ctx, tt.args.parent, tt.args.indexName)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteSpannerTableIndex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteSpannerTableIndex() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerService_SetTableIamBinding(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx     context.Context
		parent  string
		binding *TablePolicyBinding
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *TablePolicyBinding
		wantErr bool
	}{
		{
			name: "Test_SetTableIamBinding",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "tf-test", "tftest"),
				binding: &TablePolicyBinding{
					Role: "admin",
					Permissions: []TablePolicyBindingPermission{
						TablePolicyBindingPermission_SELECT,
						TablePolicyBindingPermission_INSERT,
						TablePolicyBindingPermission_UPDATE,
						TablePolicyBindingPermission_DELETE,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerService{
				GoogleCredentials: tt.fields.GoogleCredentials,
			}
			got, err := s.SetTableIamBinding(tt.args.ctx, tt.args.parent, tt.args.binding)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetTableIamBinding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetTableIamBinding() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerService_GetTableIamBinding(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx    context.Context
		parent string
		role   string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *TablePolicyBinding
		wantErr bool
	}{
		{
			name: "Test_GetTableIamBinding",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "tf-test", "tftest"),
				role:   "admin",
			},
			want:    &TablePolicyBinding{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerService{
				GoogleCredentials: tt.fields.GoogleCredentials,
			}
			got, err := s.GetTableIamBinding(tt.args.ctx, tt.args.parent, tt.args.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTableIamBinding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetTableIamBinding() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerService_DeleteTableIamBinding(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx    context.Context
		parent string
		role   string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Test_DeleteTableIamBinding",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "tf-test", "tftest"),
				role:   "admin",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerService{
				GoogleCredentials: tt.fields.GoogleCredentials,
			}
			if err := s.DeleteTableIamBinding(tt.args.ctx, tt.args.parent, tt.args.role); (err != nil) != tt.wantErr {
				t.Errorf("DeleteTableIamBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSpannerService_ListDatabaseRoles(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx       context.Context
		parent    string
		pageSize  int32
		pageToken string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*databasepb.DatabaseRole
		want1   string
		wantErr bool
	}{
		{
			name: "Test_ListDatabaseRoles",
			args: args{
				ctx:       context.Background(),
				parent:    fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "tf-test"),
				pageSize:  100,
				pageToken: "",
			},
			want:    []*databasepb.DatabaseRole{},
			want1:   "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerService{
				GoogleCredentials: tt.fields.GoogleCredentials,
			}
			got, got1, err := s.ListDatabaseRoles(tt.args.ctx, tt.args.parent, tt.args.pageSize, tt.args.pageToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListDatabaseRoles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListDatabaseRoles() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ListDatabaseRoles() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestSpannerService_DeleteDatabaseRole(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Test_DeleteDatabaseRole",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/databaseRoles/%s", TestProject, TestInstance, "tf-test", "admin"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerService{
				GoogleCredentials: tt.fields.GoogleCredentials,
			}
			if err := s.DeleteDatabaseRole(tt.args.ctx, tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("DeleteDatabaseRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateProtoBundle(t *testing.T) {
	type args struct {
		ctx              context.Context
		databaseName     string
		protoPackageName string
		descriptorSet    []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "CreateProtoBundle",
			args: args{
				ctx:              context.Background(),
				databaseName:     fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "tf-test"),
				protoPackageName: "alis.px.services.data.v2.SpannerTest",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CreateProtoBundle(tt.args.ctx, tt.args.databaseName, tt.args.protoPackageName, tt.args.descriptorSet); (err != nil) != tt.wantErr {
				t.Errorf("CreateProtoBundle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSpannerService_CreateSpannerTableForeignKeyConstraint(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx        context.Context
		parent     string
		constraint *schema.SpannerTableForeignKeyConstraint
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *schema.SpannerTableForeignKeyConstraint
		wantErr bool
	}{
		{
			name: "Test_CreateSpannerTableForeignKeyConstraint",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "alis_px_dev_cmk", "branches"),
				constraint: &schema.SpannerTableForeignKeyConstraint{
					Name:             "fk_branches_portfolio_id",
					Column:           "parent",
					ReferencedTable:  "portfolios",
					ReferencedColumn: "portfolio_id",
					OnDelete:         schema.SpannerTableConstraintActionCascade,
				},
			},
			want:    &schema.SpannerTableForeignKeyConstraint{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.CreateSpannerTableForeignKeyConstraint(tt.args.ctx, tt.args.parent, tt.args.constraint)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSpannerTableForeignKeyConstraint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateSpannerTableForeignKeyConstraint() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerService_GetSpannerTableForeignKeyConstraint(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx    context.Context
		parent string
		name   string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *schema.SpannerTableForeignKeyConstraint
		wantErr bool
	}{
		{
			name: "Test_GetSpannerTableForeignKeyConstraint",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "alis_px_dev_cmk", "branches"),
				name:   "fk_branches_portfolio_id",
			},
			want:    &schema.SpannerTableForeignKeyConstraint{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.GetSpannerTableForeignKeyConstraint(tt.args.ctx, tt.args.parent, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpannerTableForeignKeyConstraint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSpannerTableForeignKeyConstraint() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerService_CreateSpannerTableRowDeletionPolicy(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx    context.Context
		parent string
		ttl    *SpannerTableRowDeletionPolicy
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *SpannerTableRowDeletionPolicy
		wantErr bool
	}{
		{
			name: "Test_CreateSpannerTableRowDeletionPolicy",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "alis-px", "tf_test"),
				ttl: &SpannerTableRowDeletionPolicy{
					Column:   "updated_at",
					Duration: wrapperspb.Int64(1),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerService{
				GoogleCredentials: tt.fields.GoogleCredentials,
			}
			got, err := s.CreateSpannerTableRowDeletionPolicy(tt.args.ctx, tt.args.parent, tt.args.ttl)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSpannerTableRowDeletionPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateSpannerTableRowDeletionPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerService_GetSpannerTableRowDeletionPolicy(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx    context.Context
		parent string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *SpannerTableRowDeletionPolicy
		wantErr bool
	}{
		{
			name: "Test_GetSpannerTableRowDeletionPolicy",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "alis-px", "tf_test"),
			},
			want:    &SpannerTableRowDeletionPolicy{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerService{
				GoogleCredentials: tt.fields.GoogleCredentials,
			}
			got, err := s.GetSpannerTableRowDeletionPolicy(tt.args.ctx, tt.args.parent)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpannerTableRowDeletionPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSpannerTableRowDeletionPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerService_UpdateSpannerTableRowDeletionPolicy(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx    context.Context
		parent string
		ttl    *SpannerTableRowDeletionPolicy
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *SpannerTableRowDeletionPolicy
		wantErr bool
	}{
		{
			name: "Test_UpdateSpannerTableRowDeletionPolicy",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "alis-px", "tf_test"),
				ttl: &SpannerTableRowDeletionPolicy{
					Column:   "updated_at",
					Duration: wrapperspb.Int64(0),
				},
			},
			want:    &SpannerTableRowDeletionPolicy{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerService{
				GoogleCredentials: tt.fields.GoogleCredentials,
			}
			got, err := s.UpdateSpannerTableRowDeletionPolicy(tt.args.ctx, tt.args.parent, tt.args.ttl)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateSpannerTableRowDeletionPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateSpannerTableRowDeletionPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpannerService_DeleteSpannerTableRowDeletionPolicy(t *testing.T) {
	type fields struct {
		GoogleCredentials *googleoauth.Credentials
	}
	type args struct {
		ctx    context.Context
		parent string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Test_DeleteSpannerTableRowDeletionPolicy",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "alis-px", "tf_test"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SpannerService{
				GoogleCredentials: tt.fields.GoogleCredentials,
			}
			if err := s.DeleteSpannerTableRowDeletionPolicy(tt.args.ctx, tt.args.parent); (err != nil) != tt.wantErr {
				t.Errorf("DeleteSpannerTableRowDeletionPolicy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
