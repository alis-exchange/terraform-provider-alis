package services

import (
	"context"
	"slices"
	"strings"
	"testing"

	"terraform-provider-alis/internal/spanner/conn/conntest"
	"terraform-provider-alis/internal/spanner/schema"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestEmulator_ProtoBundleLifecycle drives the service against a real proto
// bundle: create, no-change re-apply, descriptor update, refusal to delete a
// type a column still references, and owned-only delete.
func TestEmulator_ProtoBundleLifecycle(t *testing.T) {
	cn, db := conntest.Setup(t, databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL)
	svc := NewSpannerService(cn)
	ctx := context.Background()
	v1 := readFixture(t, ownedIncludingPath)
	v2 := readFixture(t, ownedV2Path)
	wantOwned := []string{"tftest.v1.Kind", "tftest.v1.Simple", "tftest.v1.Simple.Nested"}

	types, ddl, err := svc.ApplyProtoBundle(ctx, db, v1, ownedPackages)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.HasPrefix(ddl, "CREATE PROTO BUNDLE") || !slices.Equal(types, wantOwned) {
		t.Fatalf("create ddl = %q, types = %v", ddl, types)
	}
	got, err := svc.GetProtoBundleTypes(ctx, db, ownedPackages)
	if err != nil || !slices.Equal(got, wantOwned) {
		t.Fatalf("GetProtoBundleTypes = %v, %v; want %v", got, err, wantOwned)
	}

	// Re-applying the same set re-sends UPDATE for the owned types and must be
	// accepted by Spanner as a no-op.
	if _, ddl, err = svc.ApplyProtoBundle(ctx, db, v1, ownedPackages); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if !strings.HasPrefix(ddl, "ALTER PROTO BUNDLE UPDATE") {
		t.Fatalf("re-apply ddl = %q, want an UPDATE of owned types", ddl)
	}

	// A changed descriptor (extra field) applies as an UPDATE.
	if _, ddl, err = svc.ApplyProtoBundle(ctx, db, v2, ownedPackages); err != nil {
		t.Fatalf("update to v2: %v", err)
	}
	if !strings.Contains(ddl, "UPDATE (") || !strings.Contains(ddl, "`tftest.v1.Simple`") {
		t.Fatalf("v2 ddl = %q, want UPDATE including tftest.v1.Simple", ddl)
	}

	// A column referencing an owned type blocks deletion.
	if err := cn.ExecuteDDL(ctx, db, "CREATE TABLE tftest_protos (id INT64, simple `tftest.v1.Simple`) PRIMARY KEY (id)"); err != nil {
		t.Fatalf("create table with PROTO column: %v", err)
	}
	err = svc.DeleteProtoBundle(ctx, db, ownedPackages)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("delete while referenced: err = %v, want FailedPrecondition", err)
	}
	if err := cn.ExecuteDDL(ctx, db, "DROP TABLE tftest_protos"); err != nil {
		t.Fatalf("drop table: %v", err)
	}

	// Owned-only delete keeps the imported tfdep.Shared in place.
	if err := svc.DeleteProtoBundle(ctx, db, ownedPackages); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetProtoBundleTypes(ctx, db, ownedPackages); status.Code(err) != codes.NotFound {
		t.Fatalf("after delete: err = %v, want NotFound", err)
	}
	statements, _, err := cn.DatabaseDdl(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	remaining := schema.ParseBundleTypes(statements)
	if _, ok := remaining["tfdep.Shared"]; !ok || len(remaining) != 1 {
		t.Fatalf("remaining bundle types = %v, want only the foreign import tfdep.Shared", remaining)
	}
}

// TestEmulator_ProtoBundleDeleteLastType pins the "bundle would become empty"
// path, where ProtoBundleDeleteDdl renders DROP PROTO BUNDLE. It also logs
// whether an ALTER that deletes every type is accepted (the emulator accepts
// it when descriptors are attached), so the DROP choice stays visible.
func TestEmulator_ProtoBundleDeleteLastType(t *testing.T) {
	cn, db := conntest.Setup(t, databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL)
	svc := NewSpannerService(cn)
	ctx := context.Background()
	packages := []string{"tftest.v1.sub"}

	if _, _, err := svc.ApplyProtoBundle(ctx, db, readFixture(t, "../schema/testdata/proto_bundle/other.fds"), packages); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Record what Spanner says about ALTER ... DELETE of the only type, so the
	// DROP decision stays evidence-based as emulator versions change.
	_, live, err := cn.DatabaseDdl(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	alterErr := cn.ExecuteDDLWithDescriptors(ctx, db, live, "ALTER PROTO BUNDLE DELETE (`tftest.v1.sub.Leaf`)")
	t.Logf("ALTER PROTO BUNDLE DELETE of the last type: err = %v", alterErr)
	if alterErr == nil {
		// The emulator accepted it; recreate so the DROP path below is still exercised.
		if _, _, err := svc.ApplyProtoBundle(ctx, db, readFixture(t, "../schema/testdata/proto_bundle/other.fds"), packages); err != nil {
			t.Fatalf("recreate: %v", err)
		}
	}

	if err := svc.DeleteProtoBundle(ctx, db, packages); err != nil {
		t.Fatalf("delete last type: %v", err)
	}
	statements, _, err := cn.DatabaseDdl(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.ParseBundleTypes(statements)) != 0 {
		t.Fatalf("bundle still present after deleting its last type: %q", statements)
	}
}

// TestEmulator_ProtoBundleDriftOutOfBand covers a type removed behind the
// resource's back: the read no longer lists it and the next apply re-inserts it.
func TestEmulator_ProtoBundleDriftOutOfBand(t *testing.T) {
	cn, db := conntest.Setup(t, databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL)
	svc := NewSpannerService(cn)
	ctx := context.Background()
	v1 := readFixture(t, ownedIncludingPath)

	if _, _, err := svc.ApplyProtoBundle(ctx, db, v1, ownedPackages); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := cn.ExecuteDDLWithDescriptors(ctx, db, v1, "ALTER PROTO BUNDLE DELETE (`tftest.v1.Kind`)"); err != nil {
		t.Fatalf("out-of-band delete: %v", err)
	}
	got, err := svc.GetProtoBundleTypes(ctx, db, ownedPackages)
	if err != nil || slices.Contains(got, "tftest.v1.Kind") {
		t.Fatalf("after drift GetProtoBundleTypes = %v, %v; want Kind gone", got, err)
	}
	_, ddl, err := svc.ApplyProtoBundle(ctx, db, v1, ownedPackages)
	if err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if !strings.Contains(ddl, "INSERT (`tftest.v1.Kind`)") {
		t.Fatalf("re-apply ddl = %q, want INSERT of the drifted type", ddl)
	}
}
