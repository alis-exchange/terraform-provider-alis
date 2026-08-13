package conn

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultRetryAttempts = 5
	defaultRetryBackoff  = 5 * time.Second
)

// DefaultRetryable is the uniform classifier: transient transport and
// concurrency errors retry; everything else surfaces immediately.
func DefaultRetryable(err error) bool {
	switch status.Code(err) {
	case codes.Aborted, codes.Unavailable, codes.ResourceExhausted:
		return true
	case codes.FailedPrecondition:
		// Spanner reports a racing schema change as FailedPrecondition; other
		// FailedPreconditions (e.g. "table not empty") are terminal.
		msg := strings.ToLower(status.Convert(err).Message())
		return strings.Contains(msg, "pending") ||
			strings.Contains(msg, "schema change") ||
			strings.Contains(msg, "concurrent")
	}
	return false
}

// WithRetry wraps any Connection — including the fake — with the uniform
// retry policy. Zero fields in p take the defaults (5 attempts, 5s initial
// backoff, DefaultRetryable). Close passes through without retry.
func WithRetry(inner Connection, p RetryPolicy) Connection {
	return &retryConn{inner: inner, policy: p.withDefaults()}
}

func (p RetryPolicy) withDefaults() RetryPolicy {
	if p.Attempts <= 0 {
		p.Attempts = defaultRetryAttempts
	}
	if p.InitialBackoff <= 0 {
		p.InitialBackoff = defaultRetryBackoff
	}
	if p.Retryable == nil {
		p.Retryable = DefaultRetryable
	}
	return p
}

type retryConn struct {
	inner  Connection
	policy RetryPolicy
}

// do runs op under the policy: backoff doubles per attempt with up to 50%
// jitter added. When ctx is cancelled mid-backoff, do returns the last
// operation error (not ctx.Err()) so callers still see a meaningful gRPC code.
func (r *retryConn) do(ctx context.Context, op func() error) error {
	backoff := r.policy.InitialBackoff
	var err error
	for attempt := 1; ; attempt++ {
		err = op()
		if err == nil || attempt >= r.policy.Attempts || !r.policy.Retryable(err) {
			return err
		}
		jitter := time.Duration(rand.Int63n(int64(backoff)/2 + 1))
		select {
		case <-ctx.Done():
			return err
		case <-time.After(backoff + jitter):
		}
		backoff *= 2
	}
}

func (r *retryConn) Dialect(ctx context.Context, database string) (Dialect, error) {
	var d Dialect
	err := r.do(ctx, func() error {
		var err error
		d, err = r.inner.Dialect(ctx, database)
		return err
	})
	return d, err
}

func (r *retryConn) ExecuteDDL(ctx context.Context, database string, statements ...string) error {
	return r.do(ctx, func() error { return r.inner.ExecuteDDL(ctx, database, statements...) })
}

func (r *retryConn) ExecuteDDLWithDescriptors(ctx context.Context, database string, protoDescriptors []byte, statements ...string) error {
	return r.do(ctx, func() error {
		return r.inner.ExecuteDDLWithDescriptors(ctx, database, protoDescriptors, statements...)
	})
}

func (r *retryConn) Exec(ctx context.Context, database, sql string, params ...any) error {
	return r.do(ctx, func() error { return r.inner.Exec(ctx, database, sql, params...) })
}

func (r *retryConn) Query(ctx context.Context, database string, dest any, sql string, params ...any) error {
	return r.do(ctx, func() error { return r.inner.Query(ctx, database, dest, sql, params...) })
}

func (r *retryConn) DatabaseRoles(ctx context.Context, database string, pageSize int32, pageToken string) ([]string, string, error) {
	var names []string
	var next string
	err := r.do(ctx, func() error {
		var err error
		names, next, err = r.inner.DatabaseRoles(ctx, database, pageSize, pageToken)
		return err
	})
	return names, next, err
}

func (r *retryConn) Close() error { return r.inner.Close() }
