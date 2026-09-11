package services

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"terraform-provider-alis/internal/spanner/conn/connfake"
	"terraform-provider-alis/internal/spanner/schema"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	protoBundleDB       = "projects/p/instances/i/databases/d"
	ownedIncludingPath  = "../schema/testdata/proto_bundle/owned_including_imports.fds"
	ownedV2Path         = "../schema/testdata/proto_bundle/owned_v2.fds"
	createAllOwnedTypes = "CREATE PROTO BUNDLE (`tfdep.Shared`, `tftest.v1.Kind`, `tftest.v1.Simple`, `tftest.v1.Simple.Nested`)"
)

var ownedPackages = []string{"tftest.v1"}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// bundleStatement renders the DDL GetDatabaseDdl would return for a bundle
// holding types, in Spanner's multi-line layout.
func bundleStatement(types ...string) string {
	quoted := make([]string, len(types))
	for i, ty := range types {
		quoted[i] = "  `" + ty + "`"
	}
	return "CREATE PROTO BUNDLE (\n" + strings.Join(quoted, ",\n") + ",\n)"
}

func ddlOps(t *testing.T, fake *connfake.Fake) []connfake.Op {
	t.Helper()
	return fake.OpsOf(connfake.OpExecuteDDL)
}

func TestApplyProtoBundle_CreateWhenEmpty(t *testing.T) {
	fake := connfake.New()
	svc := NewSpannerService(fake)
	descriptors := readFixture(t, ownedIncludingPath)

	types, ddl, err := svc.ApplyProtoBundle(context.Background(), protoBundleDB, descriptors, ownedPackages)
	if err != nil {
		t.Fatalf("ApplyProtoBundle: %v", err)
	}
	if ddl != createAllOwnedTypes {
		t.Fatalf("ddl = %q, want %q", ddl, createAllOwnedTypes)
	}
	ops := ddlOps(t, fake)
	if len(ops) != 1 || len(ops[0].Statements) != 1 || ops[0].Statements[0] != createAllOwnedTypes {
		t.Fatalf("ops = %+v, want exactly one CREATE PROTO BUNDLE", ops)
	}
	if string(ops[0].ProtoDescriptors) != string(descriptors) {
		t.Fatal("descriptor bytes were not attached to the DDL request")
	}
	want := []string{"tftest.v1.Kind", "tftest.v1.Simple", "tftest.v1.Simple.Nested"}
	if !slices.Equal(types, want) {
		t.Fatalf("types = %v, want owned %v", types, want)
	}
}

func TestApplyProtoBundle_InsertAndUpdate(t *testing.T) {
	fake := connfake.New()
	fake.SetDatabaseDdl(protoBundleDB, []string{bundleStatement("tfdep.Shared", "tftest.v1.Simple", "other.pkg.T")}, nil)
	svc := NewSpannerService(fake)

	_, ddl, err := svc.ApplyProtoBundle(context.Background(), protoBundleDB, readFixture(t, ownedIncludingPath), ownedPackages)
	if err != nil {
		t.Fatalf("ApplyProtoBundle: %v", err)
	}
	want := "ALTER PROTO BUNDLE INSERT (`tftest.v1.Kind`, `tftest.v1.Simple.Nested`) UPDATE (`tftest.v1.Simple`)"
	if ddl != want {
		t.Fatalf("ddl = %q, want %q", ddl, want)
	}
}

func TestApplyProtoBundle_DeletesOwnedAbsent(t *testing.T) {
	fake := connfake.New()
	fake.SetDatabaseDdl(protoBundleDB, []string{bundleStatement(
		"tfdep.Shared", "tftest.v1.Kind", "tftest.v1.Simple", "tftest.v1.Simple.Nested", "tftest.v1.Gone", "other.pkg.Gone",
	)}, nil)
	svc := NewSpannerService(fake)

	_, ddl, err := svc.ApplyProtoBundle(context.Background(), protoBundleDB, readFixture(t, ownedIncludingPath), ownedPackages)
	if err != nil {
		t.Fatalf("ApplyProtoBundle: %v", err)
	}
	if !strings.Contains(ddl, "DELETE (`tftest.v1.Gone`)") || strings.Contains(ddl, "other.pkg.Gone") {
		t.Fatalf("ddl = %q, want DELETE of the owned stale type only", ddl)
	}
}

func TestApplyProtoBundle_NoopSkipsDDL(t *testing.T) {
	fake := connfake.New()
	fake.SetDatabaseDdl(protoBundleDB, []string{bundleStatement(
		"tfdep.Shared", "tftest.v1.Kind", "tftest.v1.Simple", "tftest.v1.Simple.Nested",
	)}, nil)
	svc := NewSpannerService(fake)

	_, ddl, err := svc.ApplyProtoBundle(context.Background(), protoBundleDB, readFixture(t, ownedIncludingPath), ownedPackages)
	if err != nil {
		t.Fatalf("ApplyProtoBundle: %v", err)
	}
	// Owned types already present still get an UPDATE: their descriptors may
	// have changed while the type set did not. That is the whole reason the
	// resource tracks a content hash.
	want := "ALTER PROTO BUNDLE UPDATE (`tftest.v1.Kind`, `tftest.v1.Simple`, `tftest.v1.Simple.Nested`)"
	if ddl != want {
		t.Fatalf("ddl = %q, want %q", ddl, want)
	}
	if len(ddlOps(t, fake)) != 1 {
		t.Fatalf("expected one DDL op, got %d", len(ddlOps(t, fake)))
	}
}

func TestApplyProtoBundle_InvalidDescriptors(t *testing.T) {
	svc := NewSpannerService(connfake.New())
	if _, _, err := svc.ApplyProtoBundle(
		context.Background(),
		protoBundleDB,
		[]byte("junk"),
		ownedPackages,
	); status.Code(
		err,
	) != codes.InvalidArgument {
		t.Fatalf("err = %v, want InvalidArgument", err)
	}
}

func TestGetProtoBundleTypes(t *testing.T) {
	fake := connfake.New()
	svc := NewSpannerService(fake)

	t.Run("NotFound when none owned", func(t *testing.T) {
		fake.SetDatabaseDdl(protoBundleDB, []string{bundleStatement("other.pkg.T")}, nil)
		_, err := svc.GetProtoBundleTypes(context.Background(), protoBundleDB, ownedPackages)
		if status.Code(err) != codes.NotFound {
			t.Fatalf("err = %v, want NotFound", err)
		}
	})

	t.Run("NotFound when no bundle", func(t *testing.T) {
		fake.SetDatabaseDdl(protoBundleDB, []string{"CREATE TABLE t (id INT64) PRIMARY KEY (id)"}, nil)
		_, err := svc.GetProtoBundleTypes(context.Background(), protoBundleDB, ownedPackages)
		if status.Code(err) != codes.NotFound {
			t.Fatalf("err = %v, want NotFound", err)
		}
	})

	t.Run("owned subset sorted", func(t *testing.T) {
		fake.SetDatabaseDdl(protoBundleDB, []string{bundleStatement("tftest.v1.Simple", "other.pkg.T", "tftest.v1.Kind")}, nil)
		got, err := svc.GetProtoBundleTypes(context.Background(), protoBundleDB, ownedPackages)
		if err != nil {
			t.Fatalf("GetProtoBundleTypes: %v", err)
		}
		if want := []string{"tftest.v1.Kind", "tftest.v1.Simple"}; !slices.Equal(got, want) {
			t.Fatalf("types = %v, want %v", got, want)
		}
	})
}

func TestDeleteProtoBundle(t *testing.T) {
	t.Run("owned only", func(t *testing.T) {
		fake := connfake.New()
		fake.SetDatabaseDdl(protoBundleDB, []string{bundleStatement("tftest.v1.Simple", "other.pkg.T")}, nil)
		svc := NewSpannerService(fake)
		if err := svc.DeleteProtoBundle(context.Background(), protoBundleDB, ownedPackages); err != nil {
			t.Fatalf("DeleteProtoBundle: %v", err)
		}
		fake.AssertSubsequence(t, "ALTER PROTO BUNDLE DELETE (`tftest.v1.Simple`)")
	})

	t.Run("last type drops the bundle", func(t *testing.T) {
		fake := connfake.New()
		fake.SetDatabaseDdl(protoBundleDB, []string{bundleStatement("tftest.v1.Simple")}, nil)
		svc := NewSpannerService(fake)
		if err := svc.DeleteProtoBundle(context.Background(), protoBundleDB, ownedPackages); err != nil {
			t.Fatalf("DeleteProtoBundle: %v", err)
		}
		fake.AssertSubsequence(t, "DROP PROTO BUNDLE")
	})

	t.Run("nothing owned is a no-op", func(t *testing.T) {
		fake := connfake.New()
		fake.SetDatabaseDdl(protoBundleDB, []string{bundleStatement("other.pkg.T")}, nil)
		svc := NewSpannerService(fake)
		if err := svc.DeleteProtoBundle(context.Background(), protoBundleDB, ownedPackages); err != nil {
			t.Fatalf("DeleteProtoBundle: %v", err)
		}
		if len(ddlOps(t, fake)) != 0 {
			t.Fatalf("expected no DDL, got %+v", ddlOps(t, fake))
		}
	})
}

// syntheticSet writes a descriptor set whose marshalled size is about n bytes.
func syntheticSet(t *testing.T, n int) string {
	t.Helper()
	fds := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{{
		Name:    proto.String(strings.Repeat("a", n)),
		Package: proto.String("big"),
	}}}
	data, err := proto.Marshal(fds)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "big.fds")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDescriptorSources(t *testing.T) {
	svc := NewSpannerService(connfake.New())
	ctx := context.Background()

	t.Run("merges and hashes with owned types", func(t *testing.T) {
		set, err := svc.LoadDescriptorSources(ctx, []schema.DescriptorSource{{LocalPath: ownedIncludingPath}})
		if err != nil {
			t.Fatalf("LoadDescriptorSources: %v", err)
		}
		if len(set.Data) == 0 || len(set.Sha256) != 64 || set.Warning != "" {
			t.Fatalf("set = %+v, want data, a 64-char sha and no warning", set)
		}
		if _, ok := set.Types["tftest.v1.Simple"]; !ok {
			t.Fatalf("types = %v, want tftest.v1.Simple", set.Types)
		}
		v2, err := svc.LoadDescriptorSources(ctx, []schema.DescriptorSource{{LocalPath: ownedV2Path}})
		if err != nil {
			t.Fatalf("LoadDescriptorSources(v2): %v", err)
		}
		if v2.Sha256 == set.Sha256 {
			t.Fatal("v2 fixture must hash differently")
		}
	})

	t.Run("warns above 500 KiB", func(t *testing.T) {
		set, err := svc.LoadDescriptorSources(ctx, []schema.DescriptorSource{{LocalPath: syntheticSet(t, 600<<10)}})
		if err != nil {
			t.Fatalf("LoadDescriptorSources: %v", err)
		}
		if set.Warning == "" {
			t.Fatal("expected a size warning above 500 KiB")
		}
	})

	t.Run("errors above 1 MiB", func(t *testing.T) {
		_, err := svc.LoadDescriptorSources(ctx, []schema.DescriptorSource{{LocalPath: syntheticSet(t, 1<<20)}})
		if err == nil || !strings.Contains(err.Error(), "1 MiB") {
			t.Fatalf("err = %v, want an error naming the 1 MiB limit", err)
		}
	})

	t.Run("gs source without a reader fails", func(t *testing.T) {
		_, err := svc.LoadDescriptorSources(ctx, []schema.DescriptorSource{{GcsURI: "gs://b/o"}})
		if err == nil {
			t.Fatal("expected an error: service was built without a BlobReader")
		}
	})
}
