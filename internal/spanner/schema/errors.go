package schema

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrTableNotFound reports that a table does not exist in the database. It
// carries the underlying cause and surfaces as a gRPC NotFound status.
type ErrTableNotFound struct {
	table string
	err   error
}

func (e ErrTableNotFound) Error() string {
	if e.table == "" {
		return "table not found"
	}
	return fmt.Sprintf("table %q not found", e.table)
}

// Is matches any ErrTableNotFound target as well as the wrapped cause, so
// errors.Is works with both a sentinel-style ErrTableNotFound{} and the
// original driver error.
func (e ErrTableNotFound) Is(target error) bool {
	var errTableNotFound ErrTableNotFound
	return errors.As(target, &errTableNotFound) || errors.Is(e.err, target)
}

// GRPCStatus reports the error as codes.NotFound so status.Code recognizes it.
func (e ErrTableNotFound) GRPCStatus() *status.Status {
	return status.New(codes.NotFound, e.Error())
}
