package utils

import (
	"errors"

	"google.golang.org/grpc/status"
)

// ErrDetail returns a human-readable message for err, suitable for diagnostic
// details shown to practitioners. gRPC status errors are unwrapped to their
// message so users aren't shown the "rpc error: code = ... desc = ..." framing.
func ErrDetail(err error) string {
	if err == nil {
		return ""
	}
	var gs interface{ GRPCStatus() *status.Status }
	if errors.As(err, &gs) {
		return gs.GRPCStatus().Message()
	}
	return err.Error()
}
