package services

import (
	"context"
	"reflect"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateSpannerDatabase(tt.args.ctx, tt.args.parent, tt.args.databaseId, tt.args.database)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetSpannerDatabase(tt.args.ctx, tt.args.name)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UpdateSpannerDatabase(tt.args.ctx, tt.args.database, tt.args.updateMask, tt.args.allowMissing)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := ListSpannerDatabases(tt.args.ctx, tt.args.parent, tt.args.pageSize, tt.args.pageToken)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DeleteSpannerDatabase(tt.args.ctx, tt.args.name)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateSpannerTable(tt.args.ctx, tt.args.parent, tt.args.tableId, tt.args.table)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetSpannerTable(tt.args.ctx, tt.args.name)
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
		allowMissing bool
	}
	tests := []struct {
		name    string
		args    args
		want    *SpannerTable
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UpdateSpannerTable(tt.args.ctx, tt.args.table, tt.args.allowMissing)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ListSpannerTables(tt.args.ctx, tt.args.parent)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DeleteSpannerTable(tt.args.ctx, tt.args.name)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateSpannerBackup(tt.args.ctx, tt.args.parent, tt.args.backupId, tt.args.backup, tt.args.encryptionConfig)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetSpannerBackup(tt.args.ctx, tt.args.name, tt.args.readMask)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UpdateSpannerBackup(tt.args.ctx, tt.args.backup, tt.args.updateMask, tt.args.allowMissing)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := ListSpannerBackups(tt.args.ctx, tt.args.parent, tt.args.filter, tt.args.pageSize, tt.args.pageToken)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DeleteSpannerBackup(tt.args.ctx, tt.args.name)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetSpannerDatabaseIamPolicy(tt.args.ctx, tt.args.parent, tt.args.policy, tt.args.updateMask)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetSpannerDatabaseIamPolicy(tt.args.ctx, tt.args.parent, tt.args.options)
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TestSpannerDatabaseIamPermissions(tt.args.ctx, tt.args.parent, tt.args.permissions)
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
