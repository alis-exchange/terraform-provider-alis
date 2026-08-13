package spanner

import (
	"context"
	"time"
)

// withTimeout bounds ctx by d when d > 0. Resources pass 0 as the default to
// the timeouts helpers, so an unset timeout reaches here as 0 and the ctx
// passes through unbounded — configuring no timeout preserves the provider's
// original wait-forever behavior.
func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}
