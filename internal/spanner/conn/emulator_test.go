package conn_test

import (
	"context"
	"testing"

	"terraform-provider-alis/internal/spanner/conn"
	"terraform-provider-alis/internal/spanner/conn/conntest"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// TestEmulator_PortEndToEnd drives every port verb through the real GCP
// adapter against the emulator: real DDL acceptance, real INFORMATION_SCHEMA
// shapes, real scan semantics.
func TestEmulator_PortEndToEnd(t *testing.T) {
	cn, db := conntest.Setup(t, databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL)
	ctx := context.Background()

	t.Run("Dialect reports GoogleSQL and caches", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			d, err := cn.Dialect(ctx, db)
			if err != nil || d != conn.DialectGoogleSQL {
				t.Fatalf("Dialect() = %v, %v; want GoogleSQL, nil", d, err)
			}
		}
	})

	t.Run("Dialect on missing database is NotFound", func(t *testing.T) {
		_, err := cn.Dialect(ctx, "projects/test-project/instances/test-instance/databases/does-not-exist")
		if status.Code(err) != codes.NotFound {
			t.Fatalf("err = %v, want NotFound", err)
		}
	})

	t.Run("ExecuteDDL applies a batch in order", func(t *testing.T) {
		err := cn.ExecuteDDL(ctx, db,
			"CREATE TABLE singers (id INT64, name STRING(64)) PRIMARY KEY (id)",
			"CREATE INDEX singers_by_name ON singers (name)",
		)
		if err != nil {
			t.Fatalf("ExecuteDDL: %v", err)
		}
	})

	t.Run("Query reads real INFORMATION_SCHEMA shapes", func(t *testing.T) {
		type tableRow struct {
			TableName string `gorm:"column:TABLE_NAME"`
		}
		var rows []tableRow
		if err := cn.Query(ctx, db, &rows,
			"SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = ?", "singers"); err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(rows) != 1 || rows[0].TableName != "singers" {
			t.Fatalf("rows = %+v, want the singers table", rows)
		}
	})

	t.Run("Exec DML and single-row Query readback", func(t *testing.T) {
		if err := cn.Exec(ctx, db, "INSERT INTO singers (id, name) VALUES (?, ?)", 1, "Alice"); err != nil {
			t.Fatalf("Exec: %v", err)
		}

		type singer struct {
			ID   int64  `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		var got singer
		if err := cn.Query(ctx, db, &got, "SELECT id, name FROM singers WHERE id = ?", 1); err != nil {
			t.Fatalf("Query: %v", err)
		}
		if got.ID != 1 || got.Name != "Alice" {
			t.Fatalf("got %+v, want {1 Alice}", got)
		}
	})

	t.Run("single-row dest with zero rows is NotFound", func(t *testing.T) {
		type singer struct {
			ID int64 `gorm:"column:id"`
		}
		var got singer
		err := cn.Query(ctx, db, &got, "SELECT id FROM singers WHERE id = ?", 999)
		if status.Code(err) != codes.NotFound {
			t.Fatalf("err = %v, want NotFound", err)
		}
	})

	t.Run("invalid DDL surfaces a status error without retry storms", func(t *testing.T) {
		err := cn.ExecuteDDL(ctx, db, "CREATE GARBAGE nonsense")
		if err == nil {
			t.Fatal("expected error for invalid DDL")
		}
		if c := status.Code(err); c != codes.InvalidArgument && c != codes.Unknown {
			t.Fatalf("code = %v, want InvalidArgument (or Unknown from the emulator)", c)
		}
	})
}

// TestEmulator_SupportMatrix probes the features whose emulator support was
// uncertain. Unsupported features skip with the emulator's error recorded, so
// the matrix stays visible in test output as emulator versions change.
func TestEmulator_SupportMatrix(t *testing.T) {
	ctx := context.Background()

	t.Run("row deletion policy DDL and surfacing", func(t *testing.T) {
		cn, db := conntest.Setup(t, databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL)
		if err := cn.ExecuteDDL(ctx, db,
			"CREATE TABLE events (id INT64, created_at TIMESTAMP) PRIMARY KEY (id)"); err != nil {
			t.Fatalf("create table: %v", err)
		}
		if err := cn.ExecuteDDL(ctx, db,
			"ALTER TABLE events ADD ROW DELETION POLICY (OLDER_THAN(created_at, INTERVAL 30 DAY))"); err != nil {
			t.Skipf("emulator rejects ROW DELETION POLICY DDL: %v", err)
		}

		// The TTL resource only ever reads the expression back — background
		// deletion never runs on the emulator and is not needed.
		type policyRow struct {
			Expr string `gorm:"column:ROW_DELETION_POLICY_EXPRESSION"`
		}
		var row policyRow
		if err := cn.Query(ctx, db, &row,
			"SELECT ROW_DELETION_POLICY_EXPRESSION FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = ? AND ROW_DELETION_POLICY_EXPRESSION IS NOT NULL", "events"); err != nil {
			t.Fatalf("policy accepted but not surfaced in INFORMATION_SCHEMA: %v", err)
		}
		t.Logf("row deletion policy supported; expression: %s", row.Expr)
	})

	t.Run("sequences DDL and surfacing", func(t *testing.T) {
		cn, db := conntest.Setup(t, databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL)
		if err := cn.ExecuteDDL(ctx, db,
			"CREATE SEQUENCE test_seq OPTIONS (sequence_kind = 'bit_reversed_positive')"); err != nil {
			t.Skipf("emulator rejects CREATE SEQUENCE: %v", err)
		}
		type seqRow struct {
			Name string `gorm:"column:SEQUENCE_NAME"`
		}
		var rows []seqRow
		if err := cn.Query(ctx, db, &rows,
			"SELECT s.NAME AS SEQUENCE_NAME FROM INFORMATION_SCHEMA.SEQUENCES s WHERE s.NAME = ?", "test_seq"); err != nil {
			t.Fatalf("sequence accepted but INFORMATION_SCHEMA.SEQUENCES query failed: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("sequence not surfaced, rows = %+v", rows)
		}
		t.Log("sequences supported and surfaced")
	})

	t.Run("FGAC roles, grants, and DatabaseRoles listing", func(t *testing.T) {
		cn, db := conntest.Setup(t, databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL)
		if err := cn.ExecuteDDL(ctx, db,
			"CREATE TABLE inventory (id INT64) PRIMARY KEY (id)"); err != nil {
			t.Fatalf("create table: %v", err)
		}
		if err := cn.ExecuteDDL(ctx, db, "CREATE ROLE inventory_admin"); err != nil {
			t.Skipf("emulator rejects CREATE ROLE (FGAC unsupported): %v", err)
		}

		names, _, err := cn.DatabaseRoles(ctx, db, 0, "")
		if err != nil {
			t.Skipf("CREATE ROLE works but DatabaseRoles listing failed: %v", err)
		}
		t.Logf("DatabaseRoles listing: %v", names)

		if err := cn.ExecuteDDL(ctx, db, "GRANT SELECT ON TABLE inventory TO ROLE inventory_admin"); err != nil {
			t.Skipf("CREATE ROLE works but GRANT rejected: %v", err)
		}

		type privRow struct {
			Grantee string `gorm:"column:GRANTEE"`
		}
		var rows []privRow
		if err := cn.Query(ctx, db, &rows,
			"SELECT GRANTEE FROM INFORMATION_SCHEMA.TABLE_PRIVILEGES WHERE table_name = ? AND grantee = ?", "inventory", "inventory_admin"); err != nil {
			// Known emulator gap: role/grant DDL is accepted but
			// TABLE_PRIVILEGES is not surfaced, so the iam_binding read path
			// stays cloud-gated.
			t.Skipf("FGAC DDL supported but TABLE_PRIVILEGES not surfaced: %v", err)
		}
		t.Logf("FGAC fully supported; TABLE_PRIVILEGES rows for grant: %d", len(rows))
	})

	t.Run("proto bundle with descriptors", func(t *testing.T) {
		cn, db := conntest.Setup(t, databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL)

		// Build a real FileDescriptorSet from a well-known compiled-in proto.
		fd := protodesc.ToFileDescriptorProto(wrapperspb.Bool(true).ProtoReflect().Descriptor().ParentFile())
		fds, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fd}})
		if err != nil {
			t.Fatalf("marshal descriptor set: %v", err)
		}

		if err := cn.ExecuteDDLWithDescriptors(ctx, db, fds,
			"CREATE PROTO BUNDLE (`google.protobuf.BoolValue`)"); err != nil {
			t.Skipf("emulator rejects CREATE PROTO BUNDLE: %v", err)
		}
		t.Log("proto bundles supported")
	})

	t.Run("PostgreSQL dialect database", func(t *testing.T) {
		cn, db, err := conntest.TrySetup(t, databasepb.DatabaseDialect_POSTGRESQL)
		if err != nil {
			t.Skipf("emulator rejects PostgreSQL-dialect databases: %v", err)
		}
		d, err := cn.Dialect(ctx, db)
		if err != nil || d != conn.DialectPostgreSQL {
			t.Fatalf("Dialect() = %v, %v; want PostgreSQL, nil", d, err)
		}
		t.Log("PostgreSQL-dialect databases supported")
	})
}
