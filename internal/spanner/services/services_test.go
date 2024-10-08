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
					DatabaseDialect:        databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL,
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
				parent:  fmt.Sprintf("projects/%s/instances/%s/databases/%s", TestProject, TestInstance, "alis-px"),
				tableId: "tf_test",
				table: &SpannerTable{
					Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "alis-px", "tf_test"),
					Schema: &SpannerTableSchema{
						Columns: []*SpannerTableColumn{
							{
								Name:          "key",
								IsPrimaryKey:  wrapperspb.Bool(true),
								Unique:        wrapperspb.Bool(false),
								Type:          "INT64",
								Size:          wrapperspb.Int64(255),
								Required:      wrapperspb.Bool(true),
								AutoIncrement: wrapperspb.Bool(true),
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
		want    *SpannerTable
		wantErr bool
	}{
		{
			name: "Test_GetSpannerTable",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "mentenova-co", "mentenova_co_dev_62g_Maps"),
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
					Name: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "alis-px", "tf_test"),
					Schema: &SpannerTableSchema{
						Columns: []*SpannerTableColumn{
							{
								Name:          "key",
								IsPrimaryKey:  wrapperspb.Bool(true),
								Unique:        wrapperspb.Bool(false),
								Type:          "INT64",
								Size:          wrapperspb.Int64(255),
								Required:      wrapperspb.Bool(true),
								AutoIncrement: wrapperspb.Bool(true),
							},
							{
								Name:           "created_at",
								Type:           "TIMESTAMP",
								Required:       wrapperspb.Bool(false),
								AutoUpdateTime: wrapperspb.Bool(true),
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
		constraint *SpannerTableForeignKeyConstraint
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *SpannerTableForeignKeyConstraint
		wantErr bool
	}{
		{
			name: "Test_CreateSpannerTableForeignKeyConstraint",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "alis_px_dev_cmk", "branches"),
				constraint: &SpannerTableForeignKeyConstraint{
					Name:             "fk_branches_portfolio_id",
					Column:           "parent",
					ReferencedTable:  "portfolios",
					ReferencedColumn: "portfolio_id",
					OnDelete:         SpannerTableForeignKeyConstraintActionCascade,
				},
			},
			want:    &SpannerTableForeignKeyConstraint{},
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
		want    *SpannerTableForeignKeyConstraint
		wantErr bool
	}{
		{
			name: "Test_GetSpannerTableForeignKeyConstraint",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/databases/%s/tables/%s", TestProject, TestInstance, "alis_px_dev_cmk", "branches"),
				name:   "fk_branches_portfolio_id",
			},
			want:    &SpannerTableForeignKeyConstraint{},
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
