package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
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

func TestCreateSpannerDatabase(t *testing.T) {
	type args struct {
		ctx        context.Context
		parent     string
		databaseId string
		database   *databasepb.Database
	}
	tests := []struct {
		name    string
		args    args
		want    *databasepb.Database
		wantErr bool
	}{
		{
			name: "Test_CreateSpannerDatabase",
			args: args{
				ctx:        context.Background(),
				parent:     fmt.Sprintf("projects/%s/instances/%s", TestProject, TestInstance),
				databaseId: "tf-test",
				database: &databasepb.Database{
					VersionRetentionPeriod: "4h",
					DatabaseDialect:        databasepb.DatabaseDialect_POSTGRESQL,
					EnableDropProtection:   false,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.CreateSpannerDatabase(tt.args.ctx, tt.args.parent, tt.args.databaseId, tt.args.database)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSpannerDatabase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateSpannerDatabase() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestGetSpannerDatabase(t *testing.T) {
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    *databasepb.Database
		wantErr bool
	}{
		{
			name: "Test_GetSpannerDatabase",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "tf-test"),
			},
			want:    &databasepb.Database{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.GetSpannerDatabase(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpannerDatabase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSpannerDatabase() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestUpdateSpannerDatabase(t *testing.T) {
	type args struct {
		ctx          context.Context
		database     *databasepb.Database
		updateMask   *fieldmaskpb.FieldMask
		allowMissing bool
	}
	tests := []struct {
		name    string
		args    args
		want    *databasepb.Database
		wantErr bool
	}{
		{
			name: "Test_UpdateSpannerDatabase",
			args: args{
				ctx: context.Background(),
				database: &databasepb.Database{
					Name:                   fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "tf-test"),
					EnableDropProtection:   false,
					VersionRetentionPeriod: "1h",
				},
				updateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"enable_drop_protection", "version_retention_period"},
				},
				allowMissing: false,
			},
			want:    &databasepb.Database{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.UpdateSpannerDatabase(tt.args.ctx, tt.args.database, tt.args.updateMask)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateSpannerDatabase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateSpannerDatabase() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestListSpannerDatabases(t *testing.T) {
	type args struct {
		ctx       context.Context
		parent    string
		pageSize  int32
		pageToken string
	}
	tests := []struct {
		name    string
		args    args
		want    []*databasepb.Database
		want1   string
		wantErr bool
	}{
		{
			name: "Test_ListSpannerDatabases",
			args: args{
				ctx:       context.Background(),
				parent:    fmt.Sprintf("projects/%s/instances/%s", TestProject, TestInstance),
				pageSize:  1,
				pageToken: "",
			},
			want:    []*databasepb.Database{},
			want1:   "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := service.ListSpannerDatabases(tt.args.ctx, tt.args.parent, tt.args.pageSize, tt.args.pageToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListSpannerDatabases() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListSpannerDatabases() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ListSpannerDatabases() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
func TestDeleteSpannerDatabase(t *testing.T) {
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
			name: "DeleteSpannerDatabase",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "tf-test"),
			},
			want:    &emptypb.Empty{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.DeleteSpannerDatabase(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteSpannerDatabase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteSpannerDatabase() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateSpannerTable(t *testing.T) {
	type args struct {
		ctx     context.Context
		parent  string
		tableId string
		table   *SpannerTable
	}
	tests := []struct {
		name    string
		args    args
		want    *SpannerTable
		wantErr bool
	}{
		{
			name: "Test_CreateSpannerTable",
			args: args{
				ctx:     context.Background(),
				parent:  fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "alis_px_dev_cmk"),
				tableId: "tftestarr",
				table: &SpannerTable{
					Name: "tftestarr",
					Schema: &SpannerTableSchema{
						Columns: []*SpannerTableColumn{
							{
								Name:         "id",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "INT64",
								Size:         wrapperspb.Int64(255),
								Required:     wrapperspb.Bool(true),
							},
							//{
							//	Name:         "display_name",
							//	IsPrimaryKey: wrapperspb.Bool(false),
							//	Unique:       wrapperspb.Bool(false),
							//	Type:         "STRING",
							//	Size:         wrapperspb.Int64(255),
							//},
							//{
							//	Name:         "is_active",
							//	IsPrimaryKey: wrapperspb.Bool(false),
							//	Unique:       wrapperspb.Bool(false),
							//	Type:         "BOOL",
							//},
							//{
							//	Name:         "latest_return",
							//	IsPrimaryKey: wrapperspb.Bool(false),
							//	Unique:       wrapperspb.Bool(false),
							//	Type:         "FLOAT64",
							//	DefaultValue: wrapperspb.String("0.0"),
							//},
							{
								Name:         "inception_date",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "DATE",
							},
							//{
							//	Name:         "last_refreshed_at",
							//	IsPrimaryKey: wrapperspb.Bool(false),
							//	Unique:       wrapperspb.Bool(false),
							//	Type:         "TIMESTAMP",
							//},
							{
								Name:         "metadata",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "JSON",
							},
							//{
							//	Name:         "data",
							//	IsPrimaryKey: wrapperspb.Bool(false),
							//	Unique:       wrapperspb.Bool(false),
							//	Type:         "BYTES",
							//},
							//{
							//	Name:         "proto_test",
							//	IsPrimaryKey: wrapperspb.Bool(false),
							//	Unique:       wrapperspb.Bool(false),
							//	Type:         "PROTO",
							//	ProtoFileDescriptorSet: &ProtoFileDescriptorSet{
							//		ProtoPackage:                wrapperspb.String("alis.px.resources.portfolios.v1.NAVCommit"),
							//		FileDescriptorSetPath:       wrapperspb.String("gcs:gs://internal.descriptorset.alis-px-product-g51dmvo.alis.services/descriptorset.pb"),
							//		FileDescriptorSetPathSource: ProtoFileDescriptorSetSourceGcs,
							//	},
							//},
							{
								Name:         "arr_test",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "ARRAY<STRING>",
							},
							{
								Name:         "arr_test_fl32",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "ARRAY<FLOAT32>",
							},
							{
								Name:         "arr_test_fl64",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "ARRAY<FLOAT64>",
							},
							{
								Name:         "arr_test_int64",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "ARRAY<INT64>",
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
		want    *SpannerTable
		wantErr bool
	}{
		{
			name: "Test_GetSpannerTable",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "tf-test", "tftest"),
			},
			want:    &SpannerTable{},
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
		table        *SpannerTable
		updateMask   *fieldmaskpb.FieldMask
		allowMissing bool
	}
	tests := []struct {
		name    string
		args    args
		want    *SpannerTable
		wantErr bool
	}{
		{
			name: "Test_UpdateSpannerTable",
			args: args{
				ctx: context.Background(),
				table: &SpannerTable{
					Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "tf-test", "tftest"),
					Schema: &SpannerTableSchema{
						Columns: []*SpannerTableColumn{
							{
								Name:         "id",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "INT64",
								Size:         wrapperspb.Int64(255),
								Required:     wrapperspb.Bool(true),
							},
							{
								Name:         "display_name",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "STRING",
								Size:         wrapperspb.Int64(255),
							},
							{
								Name:         "is_active",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "BOOL",
							},
							{
								Name:         "latest_return",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "FLOAT64",
								DefaultValue: wrapperspb.String("0.0"),
							},
							{
								Name:         "inception_date",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "DATE",
							},
							{
								Name:         "last_refreshed_at",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "TIMESTAMP",
							},
							{
								Name:         "metadata",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "JSON",
							},
							{
								Name:         "data",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "BYTES",
							},
							//{
							//	Name:         "proto_test",
							//	IsPrimaryKey: wrapperspb.Bool(false),
							//	Unique:       wrapperspb.Bool(false),
							//	Type:         "PROTO",
							//	ProtoFileDescriptorSet: &ProtoFileDescriptorSet{
							//		ProtoPackage:                wrapperspb.String("alis.px.resources.portfolios.v1.NAVCommit"),
							//		FileDescriptorSetPath:       wrapperspb.String("gcs:gs://internal.descriptorset.alis-px-product-g51dmvo.alis.services/descriptorset.pb"),
							//		FileDescriptorSetPathSource: ProtoFileDescriptorSetSourceGcs,
							//	},
							//},
							//{
							//	Name:         "proto_test_again",
							//	IsPrimaryKey: wrapperspb.Bool(false),
							//	Unique:       wrapperspb.Bool(false),
							//	Type:         "PROTO",
							//	ProtoFileDescriptorSet: &ProtoFileDescriptorSet{
							//		ProtoPackage:                wrapperspb.String("alis.px.resources.portfolios.v1.NAVCommit"),
							//		FileDescriptorSetPath:       wrapperspb.String("gcs:gs://internal.descriptorset.alis-px-product-g51dmvo.alis.services/descriptorset.pb"),
							//		FileDescriptorSetPathSource: ProtoFileDescriptorSetSourceGcs,
							//	},
							//},
							{
								Name:         "proto_test_branch",
								IsPrimaryKey: wrapperspb.Bool(false),
								Unique:       wrapperspb.Bool(false),
								Type:         "PROTO",
								ProtoFileDescriptorSet: &ProtoFileDescriptorSet{
									ProtoPackage:                wrapperspb.String("alis.px.resources.portfolios.v1.Branch"),
									FileDescriptorSetPath:       wrapperspb.String("gcs:gs://internal.descriptorset.alis-px-product-g51dmvo.alis.services/descriptorset.pb"),
									FileDescriptorSetPathSource: ProtoFileDescriptorSetSourceGcs,
								},
							},
						},
					},
				},
				updateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"schema.columns"},
				},
				allowMissing: false,
			},
			want:    &SpannerTable{},
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
func TestListSpannerTables(t *testing.T) {
	type args struct {
		ctx    context.Context
		parent string
	}
	tests := []struct {
		name    string
		args    args
		want    []*SpannerTable
		wantErr bool
	}{
		{
			name: "Test_ListSpannerTables",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "tf-test"),
			},
			want:    []*SpannerTable{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.ListSpannerTables(tt.args.ctx, tt.args.parent)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListSpannerTables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListSpannerTables() got = %v, want %v", got, tt.want)
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
				name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "tf-test", "tftest"),
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

func TestCreateSpannerBackup(t *testing.T) {
	type args struct {
		ctx              context.Context
		parent           string
		backupId         string
		backup           *databasepb.Backup
		encryptionConfig *databasepb.CreateBackupEncryptionConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *databasepb.Backup
		wantErr bool
	}{
		{
			name: "Test_CreateSpannerBackup",
			args: args{
				ctx:      context.Background(),
				parent:   fmt.Sprintf("projects/%s/instances/%s", TestProject, TestInstance),
				backupId: "tf-test-default",
				backup: &databasepb.Backup{
					Database:    fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "tf-test"),
					VersionTime: nil,
					ExpireTime:  timestamppb.New(time.Now().Add(24 * time.Hour)),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.CreateSpannerBackup(tt.args.ctx, tt.args.parent, tt.args.backupId, tt.args.backup, tt.args.encryptionConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSpannerBackup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateSpannerBackup() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestGetSpannerBackup(t *testing.T) {
	type args struct {
		ctx      context.Context
		name     string
		readMask *fieldmaskpb.FieldMask
	}
	tests := []struct {
		name    string
		args    args
		want    *databasepb.Backup
		wantErr bool
	}{
		{
			name: "Test_GetSpannerBackup",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/backups/%s", TestProject, TestInstance, "tf-test-default"),
			},
			want:    &databasepb.Backup{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.GetSpannerBackup(tt.args.ctx, tt.args.name, tt.args.readMask)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpannerBackup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSpannerBackup() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestUpdateSpannerBackup(t *testing.T) {
	type args struct {
		ctx          context.Context
		backup       *databasepb.Backup
		updateMask   *fieldmaskpb.FieldMask
		allowMissing bool
	}
	tests := []struct {
		name    string
		args    args
		want    *databasepb.Backup
		wantErr bool
	}{
		{
			name: "Test_UpdateSpannerBackup",
			args: args{
				ctx: context.Background(),
				backup: &databasepb.Backup{
					Name:       fmt.Sprintf("projects/%s/instances/%s/backups/%s", TestProject, TestInstance, "tf-test-default"),
					ExpireTime: timestamppb.New(time.Now().Add(6 * time.Hour)),
				},
				updateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"expire_time"},
				},
				allowMissing: false,
			},
			want:    &databasepb.Backup{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.UpdateSpannerBackup(tt.args.ctx, tt.args.backup, tt.args.updateMask)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateSpannerBackup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateSpannerBackup() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestListSpannerBackups(t *testing.T) {
	type args struct {
		ctx       context.Context
		parent    string
		filter    string
		pageSize  int32
		pageToken string
	}
	tests := []struct {
		name    string
		args    args
		want    []*databasepb.Backup
		want1   string
		wantErr bool
	}{
		{
			name: "Test_ListSpannerBackups",
			args: args{
				ctx:       context.Background(),
				parent:    fmt.Sprintf("projects/%s/instances/%s", TestProject, TestInstance),
				filter:    "",
				pageSize:  1,
				pageToken: "",
			},
			want:    []*databasepb.Backup{},
			want1:   "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := service.ListSpannerBackups(tt.args.ctx, tt.args.parent, tt.args.filter, tt.args.pageSize, tt.args.pageToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListSpannerBackups() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListSpannerBackups() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ListSpannerBackups() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
func TestDeleteSpannerBackup(t *testing.T) {
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
			name: "Test_DeleteSpannerBackup",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/backups/%s", TestProject, TestInstance, "tf-test-default"),
			},
			want:    &emptypb.Empty{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.DeleteSpannerBackup(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteSpannerBackup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteSpannerBackup() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetSpannerDatabaseIamPolicy(t *testing.T) {
	type args struct {
		ctx        context.Context
		parent     string
		policy     *iampb.Policy
		updateMask *fieldmaskpb.FieldMask
	}
	tests := []struct {
		name    string
		args    args
		want    *iampb.Policy
		wantErr bool
	}{
		{
			name: "Test_SetSpannerDatabaseIamPolicy",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "tf-test"),
				policy: &iampb.Policy{
					Version: 1,
					Bindings: []*iampb.Binding{
						{
							Role:    "roles/editor",
							Members: []string{"serviceAccount:example@project.iam.gserviceaccount.com"},
						},
					},
				},
				updateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"bindings"},
				},
			},
			want:    &iampb.Policy{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.SetSpannerDatabaseIamPolicy(tt.args.ctx, tt.args.parent, tt.args.policy, tt.args.updateMask)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetSpannerDatabaseIamPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetSpannerDatabaseIamPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestGetSpannerDatabaseIamPolicy(t *testing.T) {
	type args struct {
		ctx     context.Context
		parent  string
		options *iampb.GetPolicyOptions
	}
	tests := []struct {
		name    string
		args    args
		want    *iampb.Policy
		wantErr bool
	}{
		{
			name: "Test_GetSpannerDatabaseIamPolicy",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "tf-test"),
				options: &iampb.GetPolicyOptions{
					RequestedPolicyVersion: 1,
				},
			},
			want:    &iampb.Policy{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.GetSpannerDatabaseIamPolicy(tt.args.ctx, tt.args.parent, tt.args.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpannerDatabaseIamPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSpannerDatabaseIamPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestTestSpannerDatabaseIamPermissions(t *testing.T) {
	type args struct {
		ctx         context.Context
		parent      string
		permissions []string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "Test_TestSpannerDatabaseIamPermissions",
			args: args{
				ctx:         context.Background(),
				parent:      fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "tf-test"),
				permissions: []string{"spanner.databases.get"},
			},
			want:    []string{"spanner.databases.get"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.TestSpannerDatabaseIamPermissions(tt.args.ctx, tt.args.parent, tt.args.permissions)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestSpannerDatabaseIamPermissions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TestSpannerDatabaseIamPermissions() got = %v, want %v", got, tt.want)
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
