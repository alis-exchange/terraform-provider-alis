package schema

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"terraform-provider-alis/internal/utils"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// fixture reads a compiled descriptor set from testdata/proto_bundle.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "proto_bundle", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func fixtureSet(t *testing.T, name string) *descriptorpb.FileDescriptorSet {
	t.Helper()
	fds := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(fixture(t, name), fds); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", name, err)
	}
	return fds
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func TestDescriptorTypes_WalksNestedMessagesAndEnums(t *testing.T) {
	got := sortedKeys(DescriptorTypes(fixtureSet(t, "owned_including_imports.fds")))
	want := []string{"tfdep.Shared", "tftest.v1.Kind", "tftest.v1.Simple", "tftest.v1.Simple.Nested"}
	if !slices.Equal(got, want) {
		t.Fatalf("DescriptorTypes = %v, want %v", got, want)
	}
}

func TestInPackage(t *testing.T) {
	cases := []struct {
		typeName string
		packages []string
		want     bool
	}{
		{"tftest.v1.Simple", []string{"tftest.v1"}, true},
		{"tftest.v1.Simple.Nested", []string{"tftest.v1"}, true},
		{"tftest.v1.Kind", []string{"tftest.v1"}, true},
		{"tftest.v1.sub.Leaf", []string{"tftest.v1"}, false}, // nested package, lowercase next identifier
		{"tftest.v1.Simple", []string{"tftest"}, false},      // "v1" is a package segment, not a type
		{"tfdep.Shared", []string{"tftest.v1"}, false},
		{"tfdep.Shared", []string{"tftest.v1", "tfdep"}, true},
		{"tftest.v1", []string{"tftest.v1"}, false}, // no trailing identifier at all
	}
	for _, tc := range cases {
		if got := InPackage(tc.typeName, tc.packages); got != tc.want {
			t.Errorf("InPackage(%q, %v) = %v, want %v", tc.typeName, tc.packages, got, tc.want)
		}
	}
}

func TestOwnedTypes_FiltersByPackage(t *testing.T) {
	all := DescriptorTypes(fixtureSet(t, "owned_including_imports.fds"))
	got := sortedKeys(OwnedTypes(all, []string{"tftest.v1"}))
	want := []string{"tftest.v1.Kind", "tftest.v1.Simple", "tftest.v1.Simple.Nested"}
	if !slices.Equal(got, want) {
		t.Fatalf("OwnedTypes = %v, want %v", got, want)
	}
}

func TestMergeDescriptorSets_DedupesByFileNameAndSorts(t *testing.T) {
	merged, err := MergeDescriptorSets([]utils.NamedBlob{
		{Name: "owned.fds", Data: fixture(t, "owned.fds")},
		{Name: "owned_including_imports.fds", Data: fixture(t, "owned_including_imports.fds")},
	})
	if err != nil {
		t.Fatalf("MergeDescriptorSets: %v", err)
	}
	var names []string
	for _, f := range merged.GetFile() {
		names = append(names, f.GetName())
	}
	if want := []string{"dep.proto", "owned.proto"}; !slices.Equal(names, want) {
		t.Fatalf("merged files = %v, want %v (deduped, sorted)", names, want)
	}
}

func TestMergeDescriptorSets_ConflictErrors(t *testing.T) {
	_, err := MergeDescriptorSets([]utils.NamedBlob{
		{Name: "first.fds", Data: fixture(t, "owned.fds")},
		{Name: "second.fds", Data: fixture(t, "owned_v2.fds")},
	})
	if err == nil {
		t.Fatal("expected a conflict error for owned.proto with different content")
	}
	for _, want := range []string{"owned.proto", "first.fds", "second.fds"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestMergeDescriptorSets_InvalidBlobNamesSource(t *testing.T) {
	_, err := MergeDescriptorSets([]utils.NamedBlob{{Name: "notes.txt", Data: []byte("not a descriptor set")}})
	if err == nil || !strings.Contains(err.Error(), "notes.txt") {
		t.Fatalf("err = %v, want an unmarshal error naming notes.txt", err)
	}
}

func TestMarshalDescriptorSet_Deterministic(t *testing.T) {
	set := fixtureSet(t, "owned_including_imports.fds")
	data1, sha1, err := MarshalDescriptorSet(set)
	if err != nil {
		t.Fatalf("MarshalDescriptorSet: %v", err)
	}
	data2, sha2, err := MarshalDescriptorSet(set)
	if err != nil {
		t.Fatalf("MarshalDescriptorSet: %v", err)
	}
	if string(data1) != string(data2) || sha1 != sha2 {
		t.Fatal("marshalling the same set twice produced different bytes or hashes")
	}
	if len(sha1) != 64 {
		t.Fatalf("sha = %q, want 64 hex chars", sha1)
	}
	_, shaV2, err := MarshalDescriptorSet(fixtureSet(t, "owned_v2.fds"))
	if err != nil {
		t.Fatalf("MarshalDescriptorSet(v2): %v", err)
	}
	if shaV2 == sha1 {
		t.Fatal("v2 descriptor set (extra field) must hash differently")
	}
}
