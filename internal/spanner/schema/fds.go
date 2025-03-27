package schema

import (
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// ProtoFileDescriptorSet represents a Proto File Descriptor Set.
type ProtoFileDescriptorSet struct {
	// Proto package.
	// Typically paired with PROTO columns.
	ProtoPackage *wrapperspb.StringValue
	// Proto File Descriptor Set file path.
	// Typically paired with PROTO columns.
	FileDescriptorSetPath *wrapperspb.StringValue
	// Proto File Descriptor Set file source.
	// Typically paired with PROTO columns.
	FileDescriptorSetPathSource ProtoFileDescriptorSetSource
	// Proto File Descriptor Set bytes.
	fileDescriptorSet *descriptorpb.FileDescriptorSet
}

func (f *ProtoFileDescriptorSet) SetFileDescriptorSet(fileDescriptorSet *descriptorpb.FileDescriptorSet) {
	if f != nil {
		f.fileDescriptorSet = fileDescriptorSet
	}
}

func (f *ProtoFileDescriptorSet) GetProtoPackage() *wrapperspb.StringValue {
	if f == nil {
		return nil
	}

	return f.ProtoPackage
}

func (f *ProtoFileDescriptorSet) GetFileDescriptorSetPath() *wrapperspb.StringValue {
	if f == nil {
		return nil
	}

	return f.FileDescriptorSetPath
}

func (f *ProtoFileDescriptorSet) GetFileDescriptorSetPathSource() ProtoFileDescriptorSetSource {
	if f == nil {
		return ProtoFileDescriptorSetSourceUnspecified
	}

	return f.FileDescriptorSetPathSource
}

func (f *ProtoFileDescriptorSet) GetFileDescriptorSet() *descriptorpb.FileDescriptorSet {
	if f == nil {
		return nil
	}

	return f.fileDescriptorSet
}

func (f *ProtoFileDescriptorSet) compare(other *ProtoFileDescriptorSet) bool {
	if f == nil && other == nil {
		return true
	}

	if f == nil || other == nil {
		return false
	}

	if f.GetProtoPackage().GetValue() != other.GetProtoPackage().GetValue() {
		return false
	}

	if f.GetFileDescriptorSetPath().GetValue() != other.GetFileDescriptorSetPath().GetValue() {
		return false
	}

	if f.GetFileDescriptorSetPathSource() != other.GetFileDescriptorSetPathSource() {
		return false
	}

	return true
}

type ProtoFileDescriptorSetSource int64

const (
	ProtoFileDescriptorSetSourceUnspecified ProtoFileDescriptorSetSource = iota
	ProtoFileDescriptorSetSourceGcs
	ProtoFileDescriptorSetSourceUrl
)
