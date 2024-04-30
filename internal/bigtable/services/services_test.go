package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var (
	// TestProject is the project used for testing.
	TestProject string
	// TestInstance is the instance used for testing.
	TestInstance string
	TestCluster  string
	service      *BigtableService
)

func init() {
	TestProject = os.Getenv("ALIS_OS_PROJECT")
	TestInstance = os.Getenv("ALIS_OS_INSTANCE")
	TestCluster = os.Getenv("ALIS_OS_BIGTABLE_CLUSTER")

	if TestProject == "" {
		log.Fatalf("ALIS_OS_PROJECT must be set for integration tests")
	}

	if TestInstance == "" {
		log.Fatalf("ALIS_OS_INSTANCE must be set for integration tests")
	}

	if TestCluster == "" {
		log.Fatalf("ALIS_OS_BIGTABLE_CLUSTER must be set for integration tests")
	}

	service = NewBigtableService(nil)
}

func TestCreateBigtableTable(t *testing.T) {
	type args struct {
		ctx     context.Context
		parent  string
		tableId string
		table   *BigtableTable
	}
	tests := []struct {
		name    string
		args    args
		want    *BigtableTable
		wantErr bool
	}{
		{
			name: "Test_CreateBigtableTable",
			args: args{
				ctx:     context.Background(),
				parent:  fmt.Sprintf("projects/%s/instances/%s", TestProject, TestInstance),
				tableId: "tf-test",
				table: &BigtableTable{
					Name: "",
					ColumnFamilies: map[string]bigtable.Family{
						"0": bigtable.Family{},
					},
					DeletionProtection:    bigtable.None,
					ChangeStreamRetention: 24 * time.Hour,
				},
			},
			want: &BigtableTable{
				Name: fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
				ColumnFamilies: map[string]bigtable.Family{
					"0": bigtable.Family{},
				},
				DeletionProtection:    bigtable.None,
				ChangeStreamRetention: 2 * time.Hour,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.CreateBigtableTable(tt.args.ctx, tt.args.parent, tt.args.tableId, tt.args.table)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateBigtableTable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateBigtableTable() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestGetBigtableTable(t *testing.T) {
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    *BigtableTable
		wantErr bool
	}{
		{
			name: "Test_GetBigtableTable",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
			},
			want: &BigtableTable{
				Name: fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
				ColumnFamilies: map[string]bigtable.Family{
					"0": bigtable.Family{},
				},
				DeletionProtection:    bigtable.None,
				ChangeStreamRetention: 2 * time.Hour,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.GetBigtableTable(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBigtableTable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetBigtableTable() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestUpdateBigtableTable(t *testing.T) {
	type args struct {
		ctx          context.Context
		table        *BigtableTable
		updateMask   *fieldmaskpb.FieldMask
		allowMissing bool
	}
	tests := []struct {
		name    string
		args    args
		want    *BigtableTable
		wantErr bool
	}{
		{
			name: "Test_UpdateBigtableTable",
			args: args{
				ctx: context.Background(),
				table: &BigtableTable{
					Name: fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
					ColumnFamilies: map[string]bigtable.Family{
						"0": bigtable.Family{},
					},
					DeletionProtection:    bigtable.None,
					ChangeStreamRetention: 0 * time.Hour,
				},
				updateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"change_stream_retention"},
				},
				allowMissing: false,
			},
			want:    &BigtableTable{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.UpdateBigtableTable(tt.args.ctx, tt.args.table, tt.args.updateMask, tt.args.allowMissing)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateBigtableTable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateBigtableTable() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestListBigtableTables(t *testing.T) {
	type args struct {
		ctx    context.Context
		parent string
	}
	tests := []struct {
		name    string
		args    args
		want    []*BigtableTable
		wantErr bool
	}{
		{
			name: "Test_ListBigtableTables",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s", TestProject, TestInstance),
			},
			want:    []*BigtableTable{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.ListBigtableTables(tt.args.ctx, tt.args.parent)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListBigtableTables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListBigtableTables() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestDeleteBigtableTable(t *testing.T) {
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
			name: "Test_DeleteBigtableTable",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
			},
			want:    &emptypb.Empty{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.DeleteBigtableTable(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteBigtableTable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteBigtableTable() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateBigtableColumnFamily(t *testing.T) {
	type args struct {
		ctx            context.Context
		parent         string
		columnFamilyId string
		columnFamily   *bigtable.Family
	}
	tests := []struct {
		name    string
		args    args
		want    *bigtable.Family
		wantErr bool
	}{
		{
			name: "Test_CreateBigtableColumnFamily",
			args: args{
				ctx:            context.Background(),
				parent:         fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
				columnFamilyId: "0",
				columnFamily:   &bigtable.Family{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.CreateBigtableColumnFamily(tt.args.ctx, tt.args.parent, tt.args.columnFamilyId, tt.args.columnFamily)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateBigtableColumnFamily() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateBigtableColumnFamily() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestListBigtableColumnFamilies(t *testing.T) {
	type args struct {
		ctx    context.Context
		parent string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]*bigtable.Family
		wantErr bool
	}{
		{
			name: "Test_ListBigtableColumnFamilies",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
			},
			want:    map[string]*bigtable.Family{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.ListBigtableColumnFamilies(tt.args.ctx, tt.args.parent)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListBigtableColumnFamilies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListBigtableColumnFamilies() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestDeleteBigtableColumnFamily(t *testing.T) {
	type args struct {
		ctx            context.Context
		parent         string
		columnFamilyId string
	}
	tests := []struct {
		name    string
		args    args
		want    *emptypb.Empty
		wantErr bool
	}{
		{
			name: "Test_DeleteBigtableColumnFamily",
			args: args{
				ctx:            context.Background(),
				parent:         fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
				columnFamilyId: "0",
			},
			want:    &emptypb.Empty{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.DeleteBigtableColumnFamily(tt.args.ctx, tt.args.parent, tt.args.columnFamilyId)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteBigtableColumnFamily() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteBigtableColumnFamily() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBigtableGarbageCollectionPolicy(t *testing.T) {
	type args struct {
		ctx            context.Context
		parent         string
		columnFamilyId string
	}
	tests := []struct {
		name    string
		args    args
		want    bigtable.GCPolicy
		wantErr bool
	}{
		{
			name: "Test_GetBigtableGarbageCollectionPolicy",
			args: args{
				ctx:            context.Background(),
				parent:         fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
				columnFamilyId: "0",
			},
			want:    bigtable.NoGcPolicy(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.GetBigtableGarbageCollectionPolicy(tt.args.ctx, tt.args.parent, tt.args.columnFamilyId)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBigtableGarbageCollectionPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetBigtableGarbageCollectionPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestUpdateBigtableGarbageCollectionPolicy(t *testing.T) {
	rulesString := `{"mode":"intersection","rules":[{"max_version":2},{"max_age":"4h"}]}`

	rulesMap := map[string]interface{}{}

	err := json.Unmarshal([]byte(rulesString), &rulesMap)
	if err != nil {
		t.Errorf("json.Unmarshal() error = %v", err)
		return
	}

	gcPolicy, err := GetGcPolicyFromJSON(rulesMap, true)
	if err != nil {
		t.Errorf("GetGcPolicyFromJSON() error = %v", err)
		return
	}

	type args struct {
		ctx            context.Context
		parent         string
		columnFamilyId string
		gcPolicy       *bigtable.GCPolicy
	}
	tests := []struct {
		name    string
		args    args
		want    *bigtable.GCPolicy
		wantErr bool
	}{
		{
			name: "Test_UpdateBigtableGarbageCollectionPolicy",
			args: args{
				ctx:            context.Background(),
				parent:         fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
				columnFamilyId: "0",
				gcPolicy:       &gcPolicy,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.UpdateBigtableGarbageCollectionPolicy(tt.args.ctx, tt.args.parent, tt.args.columnFamilyId, tt.args.gcPolicy)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateBigtableGarbageCollectionPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateBigtableGarbageCollectionPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestListBigtableGarbageCollectionPolicies(t *testing.T) {
	type args struct {
		ctx    context.Context
		parent string
	}
	tests := []struct {
		name    string
		args    args
		want    []*bigtable.GCPolicy
		wantErr bool
	}{
		{
			name: "Test_ListBigtableGarbageCollectionPolicies",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
			},
			want:    []*bigtable.GCPolicy{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.ListBigtableGarbageCollectionPolicies(tt.args.ctx, tt.args.parent)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListBigtableGarbageCollectionPolicies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListBigtableGarbageCollectionPolicies() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestDeleteBigtableGarbageCollectionPolicy(t *testing.T) {
	type args struct {
		ctx            context.Context
		parent         string
		columnFamilyId string
	}
	tests := []struct {
		name    string
		args    args
		want    *emptypb.Empty
		wantErr bool
	}{
		{
			name: "Test_DeleteBigtableGarbageCollectionPolicy",
			args: args{
				ctx:            context.Background(),
				parent:         fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
				columnFamilyId: "0",
			},
			want:    &emptypb.Empty{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.DeleteBigtableGarbageCollectionPolicy(tt.args.ctx, tt.args.parent, tt.args.columnFamilyId)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteBigtableGarbageCollectionPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteBigtableGarbageCollectionPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBigtableTableIamPolicy(t *testing.T) {
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
			name: "Test_GetBigtableTableIamPolicy",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
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
			got, err := service.GetBigtableTableIamPolicy(tt.args.ctx, tt.args.parent, tt.args.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBigtableTableIamPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetBigtableTableIamPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestSetBigtableTableIamPolicy(t *testing.T) {
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
			name: "Test_SetBigtableTableIamPolicy",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/tables/%s", TestProject, TestInstance, "tf-test"),
				policy: &iampb.Policy{
					Version: 1,
					Bindings: []*iampb.Binding{
						{
							Role:    "roles/bigtable.user",
							Members: []string{"serviceAccount:alis-exchange@alis-px-dev-0s6.iam.gserviceaccount.com"},
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
			got, err := service.SetBigtableTableIamPolicy(tt.args.ctx, tt.args.parent, tt.args.policy, tt.args.updateMask)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetBigtableTableIamPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetBigtableTableIamPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateBigtableBackup(t *testing.T) {
	type args struct {
		ctx      context.Context
		parent   string
		backupId string
		backup   *BigtableBackup
	}
	tests := []struct {
		name    string
		args    args
		want    *BigtableBackup
		wantErr bool
	}{
		{
			name: "Test_CreateBigtableBackup",
			args: args{
				ctx:      context.Background(),
				parent:   fmt.Sprintf("projects/%s/instances/%s/clusters/%s", TestProject, TestInstance, TestCluster),
				backupId: "tf-test-default",
				backup: &BigtableBackup{
					SourceTable: "tf-test",
					ExpireTime:  time.Now().Add(6 * time.Hour),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.CreateBigtableBackup(tt.args.ctx, tt.args.parent, tt.args.backupId, tt.args.backup)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateBigtableBackup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateBigtableBackup() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestGetBigtableBackup(t *testing.T) {
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    *BigtableBackup
		wantErr bool
	}{
		{
			name: "Test_GetBigtableBackup",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/clusters/%s/backups/%s", TestProject, TestInstance, TestCluster, "tf-test-default"),
			},
			want:    &BigtableBackup{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.GetBigtableBackup(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBigtableBackup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetBigtableBackup() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestUpdateBigtableBackup(t *testing.T) {
	type args struct {
		ctx          context.Context
		backup       *BigtableBackup
		updateMask   *fieldmaskpb.FieldMask
		allowMissing bool
	}
	tests := []struct {
		name    string
		args    args
		want    *BigtableBackup
		wantErr bool
	}{
		{
			name: "Test_UpdateBigtableBackup",
			args: args{
				ctx: context.Background(),
				backup: &BigtableBackup{
					Name:       fmt.Sprintf("projects/%s/instances/%s/clusters/%s/backups/%s", TestProject, TestInstance, TestCluster, "tf-test-default"),
					ExpireTime: time.Now().Add(12 * time.Hour),
				},
				updateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"expire_time"},
				},
			},
			want:    &BigtableBackup{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.UpdateBigtableBackup(tt.args.ctx, tt.args.backup, tt.args.updateMask, tt.args.allowMissing)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateBigtableBackup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateBigtableBackup() got = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestListBigtableBackups(t *testing.T) {
	type args struct {
		ctx       context.Context
		parent    string
		pageSize  int32
		pageToken string
	}
	tests := []struct {
		name    string
		args    args
		want    []*BigtableBackup
		want1   string
		wantErr bool
	}{
		{
			name: "Test_ListBigtableBackups",
			args: args{
				ctx:    context.Background(),
				parent: fmt.Sprintf("projects/%s/instances/%s/clusters/%s", TestProject, TestInstance, TestCluster),
			},
			want:    []*BigtableBackup{},
			want1:   "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := service.ListBigtableBackups(tt.args.ctx, tt.args.parent, tt.args.pageSize, tt.args.pageToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListBigtableBackups() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListBigtableBackups() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ListBigtableBackups() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
func TestDeleteBigtableBackup(t *testing.T) {
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
			name: "Test_DeleteBigtableBackup",
			args: args{
				ctx:  context.Background(),
				name: fmt.Sprintf("projects/%s/instances/%s/clusters/%s/backups/%s", TestProject, TestInstance, TestCluster, "tf-test-default"),
			},
			want:    &emptypb.Empty{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.DeleteBigtableBackup(tt.args.ctx, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteBigtableBackup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteBigtableBackup() got = %v, want %v", got, tt.want)
			}
		})
	}
}
