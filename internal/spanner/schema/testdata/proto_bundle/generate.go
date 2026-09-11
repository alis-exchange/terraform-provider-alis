// Package protobundle holds the compiled FileDescriptorSet fixtures for the
// proto bundle tests. Regenerate with `go generate ./...` in this directory
// after editing a .proto; protoc must be on PATH.
package protobundle

// owned_v2.fds is compiled from a copy of owned_v2.proto named owned.proto so
// that its file name collides with owned.fds and the two differ in content.

//go:generate protoc -I . --include_imports --descriptor_set_out=owned_including_imports.fds owned.proto
//go:generate protoc -I . --descriptor_set_out=owned.fds owned.proto
//go:generate protoc -I . --descriptor_set_out=other.fds other.proto
//go:generate sh -c "mkdir -p v2 && cp owned_v2.proto v2/owned.proto && cp dep.proto v2/dep.proto && protoc -I v2 --include_imports --descriptor_set_out=owned_v2.fds owned.proto && rm -r v2"
