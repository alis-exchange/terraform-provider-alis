package services

import (
	"context"
	"fmt"
	"slices"

	"terraform-provider-alis/internal/spanner/schema"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Spanner's admin API rejects requests above 1 MiB and recommends keeping
// proto bundles well below that; the descriptor set rides in the DDL request.
const (
	DescriptorsWarnBytes = 500 << 10
	DescriptorsMaxBytes  = 1 << 20
)

// DescriptorSet is a merged, deterministically marshalled descriptor set
// ready to ship as UpdateDatabaseDdl.proto_descriptors.
type DescriptorSet struct {
	Data   []byte
	Sha256 string
	// Types is every message and enum the set defines, owned or not.
	Types map[string]struct{}
	// Warning is non-empty when the set is large enough to deserve a plan
	// warning but still under the hard limit.
	Warning string
}

// LoadDescriptorSources reads, merges, and marshals every source. Local paths
// are read directly; gs:// sources need the service built WithBlobReader.
func (s *SpannerService) LoadDescriptorSources(ctx context.Context, sources []schema.DescriptorSource) (*DescriptorSet, error) {
	blobs, err := schema.LoadDescriptorSources(ctx, sources, s.blobs)
	if err != nil {
		return nil, err
	}
	merged, err := schema.MergeDescriptorSets(blobs)
	if err != nil {
		return nil, err
	}
	data, sha, err := schema.MarshalDescriptorSet(merged)
	if err != nil {
		return nil, err
	}
	set := &DescriptorSet{Data: data, Sha256: sha, Types: schema.DescriptorTypes(merged)}
	switch {
	case len(data) > DescriptorsMaxBytes:
		return nil, fmt.Errorf(
			"merged descriptor set is %d bytes, above Spanner's 1 MiB admin request limit; strip source info or split the package",
			len(data),
		)
	case len(data) > DescriptorsWarnBytes:
		set.Warning = fmt.Sprintf("merged descriptor set is %d bytes; Spanner recommends proto bundles under 500 KiB", len(data))
	}
	return set, nil
}

// ApplyProtoBundle reconciles the owned slice of database's proto bundle with
// descriptors: inserts missing types (imports included), re-sends owned types
// that already exist, and deletes owned types the set no longer defines.
// Returns the owned types the bundle holds afterwards and the DDL sent, ""
// when the bundle already matched.
func (s *SpannerService) ApplyProtoBundle(
	ctx context.Context,
	database string,
	descriptors []byte,
	packages []string,
) ([]string, string, error) {
	fds := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(descriptors, fds); err != nil {
		return nil, "", status.Errorf(codes.InvalidArgument, "descriptors are not a FileDescriptorSet: %v", err)
	}
	desired := schema.DescriptorTypes(fds)

	statements, _, err := s.conn.DatabaseDdl(ctx, database)
	if err != nil {
		return nil, "", err
	}
	current := schema.ParseBundleTypes(statements)

	diff := schema.DiffProtoBundle(current, desired, packages)
	ddl := schema.ProtoBundleDdl(len(current), diff)
	if ddl != "" {
		if err := s.conn.ExecuteDDLWithDescriptors(ctx, database, descriptors, ddl); err != nil {
			return nil, ddl, err
		}
	}
	return sortedTypes(schema.OwnedTypes(desired, packages)), ddl, nil
}

// GetProtoBundleTypes returns the owned types currently in database's proto
// bundle, sorted. codes.NotFound when the bundle holds none of them (or does
// not exist), which the resource treats as "gone".
func (s *SpannerService) GetProtoBundleTypes(ctx context.Context, database string, packages []string) ([]string, error) {
	statements, _, err := s.conn.DatabaseDdl(ctx, database)
	if err != nil {
		return nil, err
	}
	owned := schema.OwnedTypes(schema.ParseBundleTypes(statements), packages)
	if len(owned) == 0 {
		return nil, status.Errorf(codes.NotFound, "proto bundle in %s holds no types from packages %v", database, packages)
	}
	return sortedTypes(owned), nil
}

// DeleteProtoBundle removes the owned types from database's proto bundle,
// dropping the bundle when nothing else would remain. Foreign types are left
// untouched. A no-op when nothing is owned.
func (s *SpannerService) DeleteProtoBundle(ctx context.Context, database string, packages []string) error {
	statements, live, err := s.conn.DatabaseDdl(ctx, database)
	if err != nil {
		return err
	}
	current := schema.ParseBundleTypes(statements)
	owned := sortedTypes(schema.OwnedTypes(current, packages))
	ddl := schema.ProtoBundleDeleteDdl(len(current), owned)
	switch ddl {
	case "":
		return nil
	case schema.DropProtoBundleDdl:
		return s.conn.ExecuteDDL(ctx, database, ddl)
	default:
		// Spanner rejects an ALTER PROTO BUNDLE without descriptors, even for
		// a pure DELETE; the live bundle's own descriptors satisfy it.
		return s.conn.ExecuteDDLWithDescriptors(ctx, database, live, ddl)
	}
}

func sortedTypes(types map[string]struct{}) []string {
	out := make([]string, 0, len(types))
	for name := range types {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}
