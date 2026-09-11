package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"terraform-provider-alis/internal/utils"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// A Spanner database has exactly one proto bundle shared by every service
// that stores PROTO columns in it. The helpers here reconcile ONE package's
// slice of that bundle: types inside the owned packages are updated and
// deleted, types outside them (typically imports) are only ever inserted when
// missing, and everything else in the bundle is left alone. This mirrors the
// protog CLI so both tools can manage the same database without fighting.

// DescriptorTypes returns the fully qualified name of every message (nested
// ones included) and enum defined in fds.
func DescriptorTypes(fds *descriptorpb.FileDescriptorSet) map[string]struct{} {
	types := map[string]struct{}{}
	for _, file := range fds.GetFile() {
		prefix := file.GetPackage()
		if prefix != "" {
			prefix += "."
		}
		for _, enum := range file.GetEnumType() {
			types[prefix+enum.GetName()] = struct{}{}
		}
		for _, message := range file.GetMessageType() {
			collectMessageTypes(prefix, message, types)
		}
	}
	return types
}

func collectMessageTypes(prefix string, message *descriptorpb.DescriptorProto, types map[string]struct{}) {
	name := prefix + message.GetName()
	types[name] = struct{}{}
	for _, enum := range message.GetEnumType() {
		types[name+"."+enum.GetName()] = struct{}{}
	}
	for _, nested := range message.GetNestedType() {
		collectMessageTypes(name+".", nested, types)
	}
}

// InPackage reports whether typeName belongs directly to one of packages. The
// segment after the package must start with an uppercase letter: proto
// packages are lowercase, so "pkg.sub.Leaf" is a nested package's type, not
// pkg's.
func InPackage(typeName string, packages []string) bool {
	for _, pkg := range packages {
		if rest, ok := strings.CutPrefix(typeName, pkg+"."); ok && rest != "" && rest[0] >= 'A' && rest[0] <= 'Z' {
			return true
		}
	}
	return false
}

// OwnedTypes filters types down to those InPackage for packages.
func OwnedTypes(types map[string]struct{}, packages []string) map[string]struct{} {
	owned := map[string]struct{}{}
	for name := range types {
		if InPackage(name, packages) {
			owned[name] = struct{}{}
		}
	}
	return owned
}

// MergeDescriptorSets unmarshals every blob as a FileDescriptorSet and merges
// them into one set keyed by proto file name, sorted by name. The same file
// name with different content across blobs is a conflict naming both blobs:
// silently picking one would ship a bundle that disagrees with the other's
// generated code.
func MergeDescriptorSets(blobs []utils.NamedBlob) (*descriptorpb.FileDescriptorSet, error) {
	type origin struct {
		file *descriptorpb.FileDescriptorProto
		blob string
	}
	files := map[string]origin{}
	for _, blob := range blobs {
		set := &descriptorpb.FileDescriptorSet{}
		if err := proto.Unmarshal(blob.Data, set); err != nil {
			return nil, fmt.Errorf("%s is not a FileDescriptorSet: %w", blob.Name, err)
		}
		for _, file := range set.GetFile() {
			if prev, ok := files[file.GetName()]; ok {
				if !proto.Equal(prev.file, file) {
					return nil, fmt.Errorf("proto file %s differs between %s and %s", file.GetName(), prev.blob, blob.Name)
				}
				continue
			}
			files[file.GetName()] = origin{file: file, blob: blob.Name}
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	merged := &descriptorpb.FileDescriptorSet{}
	for _, name := range names {
		merged.File = append(merged.File, files[name].file)
	}
	return merged, nil
}

// MarshalDescriptorSet serialises fds deterministically and returns the bytes
// with their sha256 hex digest, which is the change signal stored in state.
func MarshalDescriptorSet(fds *descriptorpb.FileDescriptorSet) ([]byte, string, error) {
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(fds)
	if err != nil {
		return nil, "", fmt.Errorf("marshalling descriptor set: %w", err)
	}
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}

// ProtoBundleDiff is the reconciliation between the live bundle and the
// desired descriptor set for one set of owned packages. Slices are sorted so
// the rendered DDL is deterministic.
type ProtoBundleDiff struct {
	// Insert holds desired types missing from the bundle, owned or not:
	// an owned type can depend on an imported one.
	Insert []string
	// Update holds owned types already in the bundle. Their descriptors are
	// re-sent whether or not they changed; Spanner treats an identical
	// UPDATE as a no-op.
	Update []string
	// Delete holds owned types in the bundle that are no longer desired.
	Delete []string
	// Ignore holds foreign types, present or absent, that this diff leaves
	// to whoever owns them.
	Ignore []string
}

// IsEmpty reports whether the diff carries no DDL-worthy change.
func (d ProtoBundleDiff) IsEmpty() bool {
	return len(d.Insert) == 0 && len(d.Update) == 0 && len(d.Delete) == 0
}

// DiffProtoBundle computes the diff between current (types in the live
// bundle) and desired (types in the descriptor set) under packages.
func DiffProtoBundle(current, desired map[string]struct{}, packages []string) ProtoBundleDiff {
	var d ProtoBundleDiff
	for name := range desired {
		switch {
		case !hasKey(current, name):
			d.Insert = append(d.Insert, name)
		case InPackage(name, packages):
			d.Update = append(d.Update, name)
		default:
			d.Ignore = append(d.Ignore, name)
		}
	}
	for name := range current {
		if hasKey(desired, name) {
			continue
		}
		if InPackage(name, packages) {
			d.Delete = append(d.Delete, name)
		} else {
			d.Ignore = append(d.Ignore, name)
		}
	}
	slices.Sort(d.Insert)
	slices.Sort(d.Update)
	slices.Sort(d.Delete)
	slices.Sort(d.Ignore)
	return d
}

func hasKey(m map[string]struct{}, k string) bool {
	_, ok := m[k]
	return ok
}

const createProtoBundlePrefix = "CREATE PROTO BUNDLE"

// ParseBundleTypes extracts the type names from the CREATE PROTO BUNDLE
// statement in a database's DDL, as returned by GetDatabaseDdl. Spanner
// renders the live bundle as one such statement regardless of how it was
// built; INFORMATION_SCHEMA has no view of bundle types.
func ParseBundleTypes(statements []string) map[string]struct{} {
	types := map[string]struct{}{}
	for _, stmt := range statements {
		if !strings.HasPrefix(stmt, createProtoBundlePrefix) {
			continue
		}
		open := strings.Index(stmt, "(")
		closeIdx := strings.LastIndex(stmt, ")")
		if open < 0 || closeIdx <= open {
			continue
		}
		for entry := range strings.SplitSeq(stmt[open+1:closeIdx], ",") {
			name := strings.Trim(strings.TrimSpace(entry), "`")
			if name != "" {
				types[name] = struct{}{}
			}
		}
	}
	return types
}

// ProtoBundleDdl renders the single statement that applies d: CREATE when
// the bundle does not exist yet, ALTER otherwise, "" when d is empty.
func ProtoBundleDdl(existingCount int, d ProtoBundleDiff) string {
	if d.IsEmpty() {
		return ""
	}
	if existingCount == 0 {
		return createProtoBundlePrefix + " " + quotedList(d.Insert)
	}
	stmt := "ALTER PROTO BUNDLE"
	if len(d.Insert) > 0 {
		stmt += " INSERT " + quotedList(d.Insert)
	}
	if len(d.Update) > 0 {
		stmt += " UPDATE " + quotedList(d.Update)
	}
	if len(d.Delete) > 0 {
		stmt += " DELETE " + quotedList(d.Delete)
	}
	return stmt
}

// ProtoBundleDeleteDdl renders the statement removing owned from a bundle of
// existingCount types. Deleting every remaining type is not allowed through
// ALTER, so that case drops the bundle instead. "" when nothing is owned.
func ProtoBundleDeleteDdl(existingCount int, owned []string) string {
	if len(owned) == 0 {
		return ""
	}
	if len(owned) >= existingCount {
		return "DROP PROTO BUNDLE"
	}
	return "ALTER PROTO BUNDLE DELETE " + quotedList(owned)
}

func quotedList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "`" + n + "`"
	}
	return "(" + strings.Join(quoted, ", ") + ")"
}
