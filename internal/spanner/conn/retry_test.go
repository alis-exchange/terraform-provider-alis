package conn_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-alis/internal/spanner/conn"
	"terraform-provider-alis/internal/spanner/conn/connfake"
)

func fastPolicy(attempts int) conn.RetryPolicy {
	return conn.RetryPolicy{Attempts: attempts, InitialBackoff: time.Millisecond}
}

func TestWithRetry(t *testing.T) {
	ctx := context.Background()
	db := "projects/p/instances/i/databases/d"

	t.Run("retries Aborted then succeeds", func(t *testing.T) {
		fake := connfake.New()
		fake.FailNext(connfake.OpExecuteDDL, 2, status.Error(codes.Aborted, "concurrent schema change"))
		c := conn.WithRetry(fake, fastPolicy(3))

		if err := c.ExecuteDDL(ctx, db, "CREATE SEQUENCE s"); err != nil {
			t.Fatalf("ExecuteDDL after retries: %v", err)
		}
		if got := len(fake.OpsOf(connfake.OpExecuteDDL)); got != 3 {
			t.Errorf("attempts = %d, want 3 (two Aborted + one success)", got)
		}
		fake.AssertSubsequence(t, "CREATE SEQUENCE")
	})

	t.Run("exhausts attempt budget", func(t *testing.T) {
		fake := connfake.New()
		fake.FailNext(connfake.OpExecuteDDL, 99, status.Error(codes.Unavailable, "unavailable"))
		c := conn.WithRetry(fake, fastPolicy(3))

		err := c.ExecuteDDL(ctx, db, "CREATE TABLE t")
		if status.Code(err) != codes.Unavailable {
			t.Fatalf("err = %v, want Unavailable after budget exhausted", err)
		}
		if got := len(fake.OpsOf(connfake.OpExecuteDDL)); got != 3 {
			t.Errorf("attempts = %d, want exactly 3", got)
		}
	})

	t.Run("never retries InvalidArgument", func(t *testing.T) {
		fake := connfake.New()
		fake.FailNext(connfake.OpExecuteDDL, 99, status.Error(codes.InvalidArgument, "bad ddl"))
		c := conn.WithRetry(fake, fastPolicy(5))

		err := c.ExecuteDDL(ctx, db, "CREATE GARBAGE")
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("err = %v, want InvalidArgument", err)
		}
		if got := len(fake.OpsOf(connfake.OpExecuteDDL)); got != 1 {
			t.Errorf("attempts = %d, want 1 (non-retryable)", got)
		}
	})

	t.Run("retries concurrent-schema-change FailedPrecondition", func(t *testing.T) {
		fake := connfake.New()
		fake.FailNext(connfake.OpExecuteDDL, 1, status.Error(codes.FailedPrecondition, "cannot proceed: pending schema change"))
		c := conn.WithRetry(fake, fastPolicy(3))

		if err := c.ExecuteDDL(ctx, db, "DROP INDEX i"); err != nil {
			t.Fatalf("expected retry to succeed: %v", err)
		}
		if got := len(fake.OpsOf(connfake.OpExecuteDDL)); got != 2 {
			t.Errorf("attempts = %d, want 2", got)
		}
	})

	t.Run("plain FailedPrecondition is not retried", func(t *testing.T) {
		fake := connfake.New()
		fake.FailNext(connfake.OpExecuteDDL, 99, status.Error(codes.FailedPrecondition, "table has rows"))
		c := conn.WithRetry(fake, fastPolicy(5))

		if err := c.ExecuteDDL(ctx, db, "DROP TABLE t"); status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("err = %v, want FailedPrecondition passthrough", err)
		}
		if got := len(fake.OpsOf(connfake.OpExecuteDDL)); got != 1 {
			t.Errorf("attempts = %d, want 1", got)
		}
	})

	t.Run("Query retried on Unavailable and result delivered", func(t *testing.T) {
		type row struct{ Name string }
		fake := connfake.New()
		fake.OnQuery("INFORMATION_SCHEMA", []row{{Name: "x"}})
		fake.FailNext(connfake.OpQuery, 1, status.Error(codes.Unavailable, "blip"))
		c := conn.WithRetry(fake, fastPolicy(3))

		var rows []row
		if err := c.Query(ctx, db, &rows, "SELECT * FROM INFORMATION_SCHEMA.TABLES"); err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(rows) != 1 || rows[0].Name != "x" {
			t.Errorf("rows = %v, want the seeded row", rows)
		}
	})

	t.Run("single-row dest yields NotFound without retry", func(t *testing.T) {
		type row struct{ Name string }
		fake := connfake.New()
		c := conn.WithRetry(fake, fastPolicy(5))

		var r row
		err := c.Query(ctx, db, &r, "SELECT missing")
		if status.Code(err) != codes.NotFound {
			t.Fatalf("err = %v, want NotFound for empty single-row dest", err)
		}
		if got := len(fake.OpsOf(connfake.OpQuery)); got != 1 {
			t.Errorf("attempts = %d, want 1 (NotFound is not retryable)", got)
		}
	})

	t.Run("fake does not implement MetadataDB and wrapper preserves that", func(t *testing.T) {
		c := conn.WithRetry(connfake.New(), fastPolicy(1))
		if _, ok := c.(conn.MetadataDB); ok {
			t.Error("retry-wrapped fake must not satisfy MetadataDB — the gorm quarantine no-ops under fakes")
		}
	})
}
