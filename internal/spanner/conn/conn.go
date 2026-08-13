// Package conn is the Connection module: the single seam between the provider
// and Spanner. Callers speak DDL/SQL strings and plain structs; every Google
// client, credential, logger, retry policy, and session pool lives behind this
// interface. No other package in the repo may construct a Spanner admin client,
// data client, or gorm handle directly.
package conn

import (
	"context"
	"time"

	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// Dialect identifies a database's SQL dialect. It is owned by this module so
// callers and fakes never import databasepb.
type Dialect int

const (
	DialectUnknown Dialect = iota
	DialectGoogleSQL
	DialectPostgreSQL
)

// Connection is everything a service method needs to talk to Spanner.
//
// Invariants (the whole contract — callers may rely on nothing else):
//
//   - database is always the full resource name
//     "projects/{p}/instances/{i}/databases/{d}".
//   - All methods honor ctx cancellation and are safe for concurrent use.
//   - Errors are gRPC-status-compatible: status.FromError(err) yields a
//     meaningful code. Codes are preserved, never re-wrapped — callers branch
//     on codes, never on error strings or concrete Google types.
//   - Retry is NOT part of adapter implementations; wrap with WithRetry once at
//     construction. Callers never add retry loops of their own — stacked
//     policies multiply latency and replay DDL that may have partly applied.
//     A gap in what counts as retryable belongs in DefaultRetryable.
type Connection interface {
	// Dialect reports the database's SQL dialect, cached per database (a
	// database's dialect is immutable). Doubles as an existence check:
	// codes.NotFound means the database does not exist.
	Dialect(ctx context.Context, database string) (Dialect, error)

	// ExecuteDDL applies statements as ONE atomic schema-change batch
	// (UpdateDatabaseDdl + LRO wait, hidden) in slice order. Per Spanner
	// semantics a failing batch may leave a PREFIX applied — callers must
	// tolerate partial application on error. Returns only after the operation
	// fully completes. Zero statements is a no-op. Every schema change in the
	// repo goes through this method.
	ExecuteDDL(ctx context.Context, database string, statements ...string) error

	// ExecuteDDLWithDescriptors is ExecuteDDL carrying a proto
	// FileDescriptorSet — CREATE/ALTER PROTO BUNDLE statements require the
	// descriptors on the same request. Same semantics as ExecuteDDL otherwise.
	ExecuteDDLWithDescriptors(ctx context.Context, database string, protoDescriptors []byte, statements ...string) error

	// Exec runs exactly one non-schema statement (DML) with positional params.
	// Schema changes MUST use ExecuteDDL so retry and LRO semantics stay
	// uniform; passing DDL here is a contract violation.
	Exec(ctx context.Context, database, sql string, params ...any) error

	// Query runs sql with positional params and scans rows into dest.
	// Column-to-field mapping happens inside the adapter, so fakes can serve
	// canned structs.
	//
	//   dest *[]T → all rows; zero rows yields an empty slice and nil error.
	//   dest *T   → first row; zero rows yields codes.NotFound.
	Query(ctx context.Context, database string, dest any, sql string, params ...any) error

	// DatabaseRoles lists the database's role resource names (an admin
	// metadata read — roles are not reliably visible via INFORMATION_SCHEMA
	// to all principals). pageSize <= 0 lists all roles. Returns the page
	// and the next page token ("" when exhausted).
	DatabaseRoles(ctx context.Context, database string, pageSize int32, pageToken string) ([]string, string, error)

	// Close releases all cached clients and pools. Called once at provider
	// teardown; idempotent.
	Close() error
}

// Options configures the GCP adapter. The zero value is valid: ADC
// credentials, default logger, default retry policy.
type Options struct {
	// Credentials is threaded into every client the adapter creates (admin,
	// data, and the go-sql-spanner connector under gorm). nil falls back to
	// Application Default Credentials. Ignored when SPANNER_EMULATOR_HOST is
	// set.
	Credentials *googleoauth.Credentials

	// Retry is the uniform policy applied by WithRetry at construction.
	Retry RetryPolicy
}

// RetryPolicy is the uniform schema-change retry policy applied by WithRetry.
// Its main job is the parallel-apply race: concurrent terraform applies race
// their schema changes and must back off and retry.
type RetryPolicy struct {
	// Attempts is the total attempt budget. Zero means the default (5).
	Attempts int
	// InitialBackoff is doubled per attempt with jitter. Zero means 5s.
	InitialBackoff time.Duration
	// Retryable reports whether err warrants another attempt. nil means the
	// default classifier: codes.Aborted, codes.Unavailable,
	// codes.ResourceExhausted, and the FailedPrecondition
	// concurrent-schema-change family.
	Retryable func(error) bool
}

// clientOptions is the single place credentials reach clients. With
// credentials set it emits option.WithCredentials; nil emits nothing so
// clients fall back to ADC; a non-empty emulator host suppresses credentials
// entirely — the emulator ignores auth, and passing credentials alongside the
// emulator's insecure channel causes dial conflicts.
func clientOptions(creds *googleoauth.Credentials, emulatorHost string) []option.ClientOption {
	if emulatorHost != "" || creds == nil {
		return nil
	}
	return []option.ClientOption{option.WithCredentials(creds)}
}
