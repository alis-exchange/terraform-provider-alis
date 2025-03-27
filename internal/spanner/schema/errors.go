package schema

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

func (e ErrTableNotFound) Is(target error) bool {
	var errTableNotFound ErrTableNotFound
	return errors.As(target, &errTableNotFound) || errors.Is(e.err, target)
}

func (e ErrTableNotFound) GRPCStatus() *status.Status {
	return status.New(codes.NotFound, e.Error())
}
