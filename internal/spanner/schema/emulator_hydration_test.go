package schema

import (
	"context"
	"testing"

	"terraform-provider-alis/internal/spanner/conn/conntest"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// TestEmulator_TableGetHydration pins SpannerTable.Get end-to-end against the
// emulator: one table, every attribute recovered from INFORMATION_SCHEMA.
func TestEmulator_TableGetHydration(t *testing.T) {
	cn, db := conntest.Setup(t, databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL)
	ctx := context.Background()

	// Proto bundle from a compiled-in well-known type so the probe covers a
	// PROTO column without external descriptor files.
	fd := protodesc.ToFileDescriptorProto(wrapperspb.Bool(true).ProtoReflect().Descriptor().ParentFile())
	fds, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fd}})
	if err != nil {
		t.Fatalf("marshal descriptor set: %v", err)
	}
	protoOK := cn.ExecuteDDLWithDescriptors(ctx, db, fds, "CREATE PROTO BUNDLE (`google.protobuf.BoolValue`)") == nil

	if err := cn.ExecuteDDL(ctx, db,
		`CREATE TABLE hydrate_probe (
			id INT64 NOT NULL,
			label STRING(50) DEFAULT ('hello'),
			update_time TIMESTAMP OPTIONS (allow_commit_timestamp=true),
			gen INT64 AS (id + 1) STORED
		) PRIMARY KEY (id)`); err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	if protoOK {
		if err := cn.ExecuteDDL(ctx, db, "ALTER TABLE hydrate_probe ADD COLUMN pb `google.protobuf.BoolValue`"); err != nil {
			t.Logf("proto column rejected, skipping proto assertions: %v", err)
			protoOK = false
		}
	}

	got, err := (&SpannerTable{}).Get(ctx, cn, db+"/tables/hydrate_probe")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	cols := map[string]*SpannerTableColumn{}
	for _, c := range got.GetSchema().GetColumns() {
		cols[c.Name] = c
	}

	if c := cols["id"]; c.Type != "INT64" || !c.GetRequired().GetValue() || !c.GetIsPrimaryKey().GetValue() {
		t.Errorf("id = %+v, want INT64, required, primary key", c)
	}
	if c := cols["label"]; c.Type != "STRING" || c.GetSize().GetValue() != 50 || c.GetDefaultValue().GetValue() != "'hello'" {
		t.Errorf("label = %+v, want STRING(50) default 'hello'", c)
	}
	if c := cols["update_time"]; c.AutoUpdateTime == nil || !c.AutoUpdateTime.GetValue() {
		t.Errorf("update_time.AutoUpdateTime = %v, want true from COLUMN_OPTIONS", c.AutoUpdateTime)
	}
	if c := cols["gen"]; !c.GetIsComputed().GetValue() || c.GetComputationDdl().GetValue() != "id + 1" || !c.GetIsStored().GetValue() {
		t.Errorf("gen = %+v, want stored generated column with expression id + 1", c)
	}
	if protoOK {
		c := cols["pb"]
		if c.Type != "PROTO" || c.GetProtoPackage().GetValue() != "google.protobuf.BoolValue" {
			t.Errorf("pb = %+v, want PROTO with package google.protobuf.BoolValue", c)
		}
	}
}
