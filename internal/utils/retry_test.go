package utils

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestRetry(t *testing.T) {
	type args[R interface{}] struct {
		attempts     int
		initialSleep time.Duration
		f            func() (R, error)
	}
	type testCase[R interface{}] struct {
		name    string
		args    args[R]
		want    R
		wantErr bool
	}
	tests := []testCase[interface{}]{
		{
			name: "Test Retry",
			args: args[interface{}]{
				attempts:     2,
				initialSleep: 5 * time.Second,
				f: func() (interface{}, error) {
					return "test", fmt.Errorf("error")
				},
			},
			want:    "test",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tStart := time.Now()
			got, err := Retry(tt.args.attempts, tt.args.initialSleep, tt.args.f)
			tEnd := time.Now()
			fmt.Println("Time taken: ", tEnd.Sub(tStart))
			if (err != nil) != tt.wantErr {
				t.Errorf("Retry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Retry() got = %v, want %v", got, tt.want)
			}
		})
	}
}
