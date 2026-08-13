package utils

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrDetail(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil",
			err:  nil,
			want: "",
		},
		{
			name: "plain error",
			err:  errors.New("something broke"),
			want: "something broke",
		},
		{
			name: "grpc status error",
			err:  status.Error(codes.InvalidArgument, "field is required but not provided"),
			want: "field is required but not provided",
		},
		{
			name: "wrapped grpc status error",
			err:  fmt.Errorf("calling service: %w", status.Error(codes.NotFound, "table not found")),
			want: "table not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ErrDetail(tt.err); got != tt.want {
				t.Errorf("ErrDetail() = %q, want %q", got, tt.want)
			}
		})
	}
}
