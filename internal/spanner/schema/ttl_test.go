package schema

import (
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestSpannerTableRowDeletionPolicyDdl(t *testing.T) {
	policy := &SpannerTableRowDeletionPolicy{
		Column:   "created_at",
		Duration: wrapperspb.Int64(30),
	}

	t.Run("CreateDdl", func(t *testing.T) {
		got, err := policy.CreateDdl("events")
		want := "ALTER TABLE events ADD ROW DELETION POLICY (OLDER_THAN(created_at, INTERVAL 30 DAY))"
		if err != nil || got != want {
			t.Errorf("CreateDdl() = (%q, %v), want %q", got, err, want)
		}
	})

	t.Run("ReplaceDdl", func(t *testing.T) {
		got, err := policy.ReplaceDdl("events")
		want := "ALTER TABLE events REPLACE ROW DELETION POLICY (OLDER_THAN(created_at, INTERVAL 30 DAY))"
		if err != nil || got != want {
			t.Errorf("ReplaceDdl() = (%q, %v), want %q", got, err, want)
		}
	})

	t.Run("DropRowDeletionPolicyDdl", func(t *testing.T) {
		if got := DropRowDeletionPolicyDdl("events"); got != "ALTER TABLE events DROP ROW DELETION POLICY" {
			t.Errorf("DropRowDeletionPolicyDdl() = %q", got)
		}
	})

	t.Run("missing table errors", func(t *testing.T) {
		if _, err := policy.CreateDdl(""); err == nil {
			t.Error("CreateDdl with empty table should error")
		}
	})

	t.Run("missing column errors", func(t *testing.T) {
		p := &SpannerTableRowDeletionPolicy{Duration: wrapperspb.Int64(1)}
		if _, err := p.CreateDdl("events"); err == nil {
			t.Error("CreateDdl without column should error")
		}
	})

	t.Run("missing duration errors", func(t *testing.T) {
		p := &SpannerTableRowDeletionPolicy{Column: "c"}
		if _, err := p.CreateDdl("events"); err == nil {
			t.Error("CreateDdl without duration should error")
		}
	})

	t.Run("nil receiver returns empty", func(t *testing.T) {
		var p *SpannerTableRowDeletionPolicy
		got, err := p.CreateDdl("events")
		if got != "" || err != nil {
			t.Errorf("nil receiver = (%q, %v), want empty", got, err)
		}
	})
}
