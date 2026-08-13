package schema

import (
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

// computedColumn returns a valid computed column; tests vary IsStored on copies of it.
func computedColumn() *SpannerTableColumn {
	return &SpannerTableColumn{
		Name:           "full_name",
		Type:           "STRING(MAX)",
		IsComputed:     wrapperspb.Bool(true),
		ComputationDdl: wrapperspb.String("CONCAT(first_name, ' ', last_name)"),
	}
}

func TestSpannerTableColumnCompare_IsStored(t *testing.T) {
	tests := []struct {
		name      string
		prior     *wrapperspb.BoolValue
		planned   *wrapperspb.BoolValue
		wantEqual bool
	}{
		{"true vs nil differs", wrapperspb.Bool(true), nil, false},
		{"nil vs true differs", nil, wrapperspb.Bool(true), false},
		{"true vs false differs", wrapperspb.Bool(true), wrapperspb.Bool(false), false},
		{"nil vs false equal (null means false)", nil, wrapperspb.Bool(false), true},
		{"true vs true equal", wrapperspb.Bool(true), wrapperspb.Bool(true), true},
		{"nil vs nil equal", nil, nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prior, planned := computedColumn(), computedColumn()
			prior.IsStored = tc.prior
			planned.IsStored = tc.planned
			if got := prior.compare(planned); got != tc.wantEqual {
				t.Errorf("compare() = %v, want %v (IsStored prior=%v planned=%v)",
					got, tc.wantEqual, tc.prior.GetValue(), tc.planned.GetValue())
			}
		})
	}
}
