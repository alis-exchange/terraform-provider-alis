package utils

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// MessageFromFileDescriptorSet creates a new dynamic proto.Message from a file descriptor set
func MessageFromFileDescriptorSet(protoPackageName string, fds *descriptorpb.FileDescriptorSet) (proto.Message, error) {

	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, err
	}

	desc, err := files.FindDescriptorByName(protoreflect.FullName(protoPackageName))
	if err != nil {
		return nil, err
	}

	var protoMsg proto.Message
	protoMsg = dynamicpb.NewMessage(desc.(protoreflect.MessageDescriptor))

	return protoMsg, nil
}
