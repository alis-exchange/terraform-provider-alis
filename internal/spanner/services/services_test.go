package services

import (
	"context"
	"slices"
	"testing"

	"terraform-provider-alis/internal/spanner/conn"
	"terraform-provider-alis/internal/spanner/conn/conntest"
	"terraform-provider-alis/internal/spanner/schema"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// IntegrationSuite runs full lifecycles (create → read → mutate → delete)
// for every Spanner resource the provider manages, against the backend
// resolved once by conntest.Target: an emulator by default, falling back to
// live Spanner when no emulator is available and GOOGLE_PROJECT /
// SPANNER_INSTANCE are set. Each test creates the objects it needs under
// distinct tftest_-prefixed names and removes them afterwards, so a shared
// live database is left as it was found.
type IntegrationSuite struct {
	suite.Suite

	ctx     context.Context
	service *SpannerService
	cn      conn.Connection
	db      string
	live    bool
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationSuite))
}

func (s *IntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	// Target registers its cleanups (connection close, emulator database
	// drop) on the suite-level T, so they run after the last test.
	s.cn, s.db, s.live = conntest.Target(s.T())
	s.service = NewSpannerService(s.cn)
}

// createTable creates a table as a lifecycle prerequisite and registers a
// cleanup on the current test that drops it.
func (s *IntegrationSuite) createTable(tableID string, columns []*schema.SpannerTableColumn) {
	s.T().Helper()
	_, err := s.service.CreateSpannerTable(s.ctx, s.db, tableID, &schema.SpannerTable{
		Schema: &schema.SpannerTableSchema{Columns: columns},
	})
	s.Require().NoError(err, "create table %s", tableID)
	s.T().Cleanup(func() {
		_, _ = s.service.DeleteSpannerTable(context.Background(), s.db+"/tables/"+tableID)
	})
}

// ensureBoolValueBundle provisions a proto bundle for the compiled-in
// google.protobuf.BoolValue message so emulator runs can host PROTO columns
// without external descriptor files.
func (s *IntegrationSuite) ensureBoolValueBundle() {
	s.T().Helper()
	fd := protodesc.ToFileDescriptorProto(wrapperspb.Bool(true).ProtoReflect().Descriptor().ParentFile())
	fds, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fd}})
	s.Require().NoError(err, "marshal descriptor set")
	s.Require().NoError(
		s.cn.ExecuteDDLWithDescriptors(s.ctx, s.db, fds, "CREATE PROTO BUNDLE (`google.protobuf.BoolValue`)"),
		"create proto bundle")
}

func idColumn() *schema.SpannerTableColumn {
	return &schema.SpannerTableColumn{
		Name:         "id",
		Type:         "INT64",
		IsPrimaryKey: wrapperspb.Bool(true),
		Required:     wrapperspb.Bool(true),
	}
}

func (s *IntegrationSuite) TestDatabaseRoleLifecycle() {
	roleID := "tftest_admin"
	roleName := s.db + "/databaseRoles/" + roleID

	role, err := s.service.CreateDatabaseRole(s.ctx, s.db, roleID)
	s.Require().NoError(err, "CreateDatabaseRole")
	s.T().Cleanup(func() { _ = s.service.DeleteDatabaseRole(context.Background(), roleName) })
	s.Equal(roleName, role.GetName())

	// Role reads go through the ListDatabaseRoles admin API, which the
	// emulator does not implement; DDL-side create/delete is still covered.
	readable := true
	got, err := s.service.GetDatabaseRole(s.ctx, roleName)
	if !s.live && status.Code(err) == codes.Unimplemented {
		readable = false
		s.T().Log("emulator does not implement the ListDatabaseRoles admin API; skipping read-back assertions")
	} else {
		s.Require().NoError(err, "GetDatabaseRole")
		s.Equal(roleName, got.GetName())
	}

	if readable {
		roles, _, err := s.service.ListDatabaseRoles(s.ctx, s.db, 0, "")
		s.Require().NoError(err, "ListDatabaseRoles")
		names := make([]string, 0, len(roles))
		for _, r := range roles {
			names = append(names, r.GetName())
		}
		s.Contains(names, roleName)
	}

	s.Require().NoError(s.service.DeleteDatabaseRole(s.ctx, roleName), "DeleteDatabaseRole")
	if readable {
		_, err := s.service.GetDatabaseRole(s.ctx, roleName)
		s.Equal(codes.NotFound, status.Code(err), "GetDatabaseRole after delete")
	}
}

func (s *IntegrationSuite) TestTableLifecycle() {
	tableID := "tftest_table"
	tableName := s.db + "/tables/" + tableID

	columns := []*schema.SpannerTableColumn{
		idColumn(),
		{Name: "display_name", Type: "STRING", Size: wrapperspb.Int64(255)},
		{Name: "is_active", Type: "BOOL"},
		{Name: "latest_return", Type: "FLOAT64", DefaultValue: wrapperspb.String("0.0")},
		{Name: "update_time", Type: "TIMESTAMP", AutoUpdateTime: wrapperspb.Bool(true)},
		{Name: "metadata", Type: "JSON"},
		{Name: "data", Type: "BYTES"},
		{Name: "tags", Type: "ARRAY<STRING>"},
	}
	// The emulator database is provisioned from scratch, so the test can
	// guarantee a proto bundle and cover a PROTO column; a live database's
	// bundles are unknown, so live runs skip that column.
	if !s.live {
		s.ensureBoolValueBundle()
		columns = append(columns, &schema.SpannerTableColumn{
			Name:         "pb",
			Type:         "PROTO",
			ProtoPackage: wrapperspb.String("google.protobuf.BoolValue"),
		})
	}

	s.createTable(tableID, columns)

	got, err := s.service.GetSpannerTable(s.ctx, tableName)
	s.Require().NoError(err, "GetSpannerTable")
	byName := map[string]*schema.SpannerTableColumn{}
	for _, c := range got.GetSchema().GetColumns() {
		byName[c.Name] = c
	}
	s.Len(byName, len(columns))

	hydration := []struct {
		name  string
		check func(c *schema.SpannerTableColumn)
	}{
		{name: "id", check: func(c *schema.SpannerTableColumn) {
			s.True(c.GetIsPrimaryKey().GetValue(), "primary key")
			s.True(c.GetRequired().GetValue(), "required")
		}},
		{name: "display_name", check: func(c *schema.SpannerTableColumn) {
			s.Equal("STRING", c.GetType())
			s.EqualValues(255, c.GetSize().GetValue())
		}},
		{name: "latest_return", check: func(c *schema.SpannerTableColumn) {
			s.Equal("0.0", c.GetDefaultValue().GetValue())
		}},
		{name: "update_time", check: func(c *schema.SpannerTableColumn) {
			s.True(c.GetAutoUpdateTime().GetValue(), "auto_update_time")
		}},
		{name: "tags", check: func(c *schema.SpannerTableColumn) {
			s.Equal("ARRAY<STRING>", c.GetType())
		}},
	}
	if !s.live {
		hydration = append(hydration, struct {
			name  string
			check func(c *schema.SpannerTableColumn)
		}{name: "pb", check: func(c *schema.SpannerTableColumn) {
			s.Equal("PROTO", c.GetType())
			s.Equal("google.protobuf.BoolValue", c.GetProtoPackage().GetValue())
		}})
	}
	for _, tc := range hydration {
		s.Run("hydrates "+tc.name, func() {
			c, ok := byName[tc.name]
			s.Require().True(ok, "column %s missing", tc.name)
			tc.check(c)
		})
	}

	// Update: widen display_name and add a column through the schema.columns
	// field mask.
	updated := slices.Clone(columns)
	for i, c := range updated {
		if c.Name == "display_name" {
			updated[i] = &schema.SpannerTableColumn{Name: "display_name", Type: "STRING", Size: wrapperspb.Int64(500)}
		}
	}
	updated = append(updated, &schema.SpannerTableColumn{Name: "notes", Type: "STRING", Size: wrapperspb.Int64(100)})

	_, err = s.service.UpdateSpannerTable(s.ctx,
		&schema.SpannerTable{Name: tableName, Schema: &schema.SpannerTableSchema{Columns: updated}},
		&fieldmaskpb.FieldMask{Paths: []string{"schema.columns"}}, false)
	s.Require().NoError(err, "UpdateSpannerTable")

	got, err = s.service.GetSpannerTable(s.ctx, tableName)
	s.Require().NoError(err, "GetSpannerTable after update")
	byName = map[string]*schema.SpannerTableColumn{}
	for _, c := range got.GetSchema().GetColumns() {
		byName[c.Name] = c
	}
	s.Require().Contains(byName, "notes", "added column missing after update")
	s.EqualValues(500, byName["display_name"].GetSize().GetValue(), "display_name size after update")

	_, err = s.service.DeleteSpannerTable(s.ctx, tableName)
	s.Require().NoError(err, "DeleteSpannerTable")
	_, err = s.service.GetSpannerTable(s.ctx, tableName)
	s.Equal(codes.NotFound, status.Code(err), "GetSpannerTable after delete")
}

func (s *IntegrationSuite) TestTableIndexLifecycle() {
	tableID := "tftest_idx"
	tableName := s.db + "/tables/" + tableID
	indexName := "tftest_idx_by_name"

	s.createTable(tableID, []*schema.SpannerTableColumn{
		idColumn(),
		{Name: "display_name", Type: "STRING", Size: wrapperspb.Int64(255)},
	})

	_, err := s.service.CreateSpannerTableIndex(s.ctx, tableName, &SpannerTableIndex{
		Name: indexName,
		Columns: []*SpannerTableIndexColumn{
			{Name: "display_name", Order: SpannerTableIndexColumnOrder_DESC},
		},
		Unique: wrapperspb.Bool(true),
	})
	s.Require().NoError(err, "CreateSpannerTableIndex")

	got, err := s.service.GetSpannerTableIndex(s.ctx, tableName, indexName)
	s.Require().NoError(err, "GetSpannerTableIndex")
	s.Require().Len(got.Columns, 1)
	s.Equal("display_name", got.Columns[0].Name)
	s.Equal(SpannerTableIndexColumnOrder_DESC, got.Columns[0].Order)
	s.True(got.Unique.GetValue(), "unique")

	indices, err := s.service.ListSpannerTableIndices(s.ctx, tableName)
	s.Require().NoError(err, "ListSpannerTableIndices")
	names := make([]string, 0, len(indices))
	for _, idx := range indices {
		names = append(names, idx.Name)
	}
	s.Contains(names, indexName)

	_, err = s.service.DeleteSpannerTableIndex(s.ctx, tableName, indexName)
	s.Require().NoError(err, "DeleteSpannerTableIndex")
	_, err = s.service.GetSpannerTableIndex(s.ctx, tableName, indexName)
	s.Equal(codes.NotFound, status.Code(err), "GetSpannerTableIndex after delete")
}

func (s *IntegrationSuite) TestForeignKeyLifecycle() {
	childName := s.db + "/tables/tftest_fk_orders"
	constraintName := "FK_tftest_orders_user"

	s.createTable("tftest_fk_users", []*schema.SpannerTableColumn{idColumn()})
	s.createTable("tftest_fk_orders", []*schema.SpannerTableColumn{
		idColumn(),
		{Name: "user_id", Type: "INT64"},
	})

	_, err := s.service.CreateSpannerTableForeignKeyConstraint(s.ctx, childName, &schema.SpannerTableForeignKeyConstraint{
		Name:             constraintName,
		Column:           "user_id",
		ReferencedTable:  "tftest_fk_users",
		ReferencedColumn: "id",
	})
	s.Require().NoError(err, "CreateSpannerTableForeignKeyConstraint")
	// The constraint must be dropped before the tables it links can be.
	s.T().Cleanup(func() {
		_ = s.service.DeleteSpannerTableForeignKeyConstraint(context.Background(), childName, constraintName)
	})

	got, err := s.service.GetSpannerTableForeignKeyConstraint(s.ctx, childName, constraintName)
	s.Require().NoError(err, "GetSpannerTableForeignKeyConstraint")
	s.Equal("user_id", got.Column)
	s.Equal("tftest_fk_users", got.ReferencedTable)
	s.Equal("id", got.ReferencedColumn)

	s.Require().NoError(
		s.service.DeleteSpannerTableForeignKeyConstraint(s.ctx, childName, constraintName),
		"DeleteSpannerTableForeignKeyConstraint")
	_, err = s.service.GetSpannerTableForeignKeyConstraint(s.ctx, childName, constraintName)
	s.Equal(codes.NotFound, status.Code(err), "GetSpannerTableForeignKeyConstraint after delete")
}

func (s *IntegrationSuite) TestRowDeletionPolicyLifecycle() {
	tableID := "tftest_ttl"
	tableName := s.db + "/tables/" + tableID

	s.createTable(tableID, []*schema.SpannerTableColumn{
		idColumn(),
		{Name: "created_at", Type: "TIMESTAMP"},
	})

	_, err := s.service.CreateSpannerTableRowDeletionPolicy(s.ctx, tableName, &SpannerTableRowDeletionPolicy{
		Column:   "created_at",
		Duration: wrapperspb.Int64(30),
	})
	s.Require().NoError(err, "CreateSpannerTableRowDeletionPolicy")

	got, err := s.service.GetSpannerTableRowDeletionPolicy(s.ctx, tableName)
	s.Require().NoError(err, "GetSpannerTableRowDeletionPolicy")
	s.Equal("created_at", got.Column)
	s.EqualValues(30, got.Duration.GetValue())

	_, err = s.service.UpdateSpannerTableRowDeletionPolicy(s.ctx, tableName, &SpannerTableRowDeletionPolicy{
		Column:   "created_at",
		Duration: wrapperspb.Int64(60),
	})
	s.Require().NoError(err, "UpdateSpannerTableRowDeletionPolicy")
	got, err = s.service.GetSpannerTableRowDeletionPolicy(s.ctx, tableName)
	s.Require().NoError(err, "GetSpannerTableRowDeletionPolicy after update")
	s.EqualValues(60, got.Duration.GetValue())

	s.Require().NoError(s.service.DeleteSpannerTableRowDeletionPolicy(s.ctx, tableName), "DeleteSpannerTableRowDeletionPolicy")
	_, err = s.service.GetSpannerTableRowDeletionPolicy(s.ctx, tableName)
	s.Error(err, "GetSpannerTableRowDeletionPolicy after delete should fail")
}

func (s *IntegrationSuite) TestSequenceLifecycle() {
	seqName := s.db + "/sequences/tftest_seq"

	_, err := s.service.CreateSpannerSequence(s.ctx, s.db, &schema.SpannerSequence{
		Name: seqName,
		Options: &schema.SpannerSequenceOptions{
			SequenceKind: schema.SpannerSequenceKindBitReversedPositive,
		},
	})
	s.Require().NoError(err, "CreateSpannerSequence")
	s.T().Cleanup(func() { _ = s.service.DeleteSpannerSequence(context.Background(), seqName) })

	got, err := s.service.GetSpannerSequence(s.ctx, seqName)
	s.Require().NoError(err, "GetSpannerSequence")
	s.Equal(seqName, got.GetName())

	_, err = s.service.UpdateSpannerSequence(s.ctx, &schema.SpannerSequence{
		Name: seqName,
		Options: &schema.SpannerSequenceOptions{
			SequenceKind: schema.SpannerSequenceKindBitReversedPositive,
			SkipRange: &schema.SpannerSequenceSkipRange{
				Min: wrapperspb.Int64(1),
				Max: wrapperspb.Int64(1000),
			},
		},
	})
	s.Require().NoError(err, "UpdateSpannerSequence")
	got, err = s.service.GetSpannerSequence(s.ctx, seqName)
	s.Require().NoError(err, "GetSpannerSequence after update")
	s.EqualValues(1, got.GetOptions().GetSkipRange().GetMin().GetValue())
	s.EqualValues(1000, got.GetOptions().GetSkipRange().GetMax().GetValue())

	s.Require().NoError(s.service.DeleteSpannerSequence(s.ctx, seqName), "DeleteSpannerSequence")
	_, err = s.service.GetSpannerSequence(s.ctx, seqName)
	s.Equal(codes.NotFound, status.Code(err), "GetSpannerSequence after delete")
}

func (s *IntegrationSuite) TestTableIamBindingLifecycle() {
	if !s.live {
		s.T().Skip("emulator does not surface INFORMATION_SCHEMA.TABLE_PRIVILEGES; IAM binding reads need live Spanner")
	}
	tableID := "tftest_iam"
	tableName := s.db + "/tables/" + tableID
	roleID := "tftest_reader"
	roleName := s.db + "/databaseRoles/" + roleID

	s.createTable(tableID, []*schema.SpannerTableColumn{idColumn()})
	_, err := s.service.CreateDatabaseRole(s.ctx, s.db, roleID)
	s.Require().NoError(err, "CreateDatabaseRole")
	s.T().Cleanup(func() { _ = s.service.DeleteDatabaseRole(context.Background(), roleName) })

	_, err = s.service.SetTableIamBinding(s.ctx, tableName, &TablePolicyBinding{
		Role:        roleID,
		Permissions: []TablePolicyBindingPermission{TablePolicyBindingPermission_SELECT},
	})
	s.Require().NoError(err, "SetTableIamBinding")
	s.T().Cleanup(func() { _ = s.service.DeleteTableIamBinding(context.Background(), tableName, roleID) })

	got, err := s.service.GetTableIamBinding(s.ctx, tableName, roleID)
	s.Require().NoError(err, "GetTableIamBinding")
	s.Equal(roleID, got.Role)
	s.Contains(got.Permissions, TablePolicyBindingPermission_SELECT)

	s.Require().NoError(s.service.DeleteTableIamBinding(s.ctx, tableName, roleID), "DeleteTableIamBinding")
}
