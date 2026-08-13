// Package conntest wires tests to a real Spanner emulator so the production
// GCP adapter can be exercised end to end: real DDL acceptance, real
// INFORMATION_SCHEMA shapes, real LRO completion.
//
// Resolution order: an already-set SPANNER_EMULATOR_HOST wins (CI service
// container or a developer's own emulator); otherwise a container is started
// via testcontainers-go when Docker is available (image
// gcr.io/cloud-spanner-emulator/emulator:latest, overridable via
// SPANNER_EMULATOR_IMAGE); otherwise the test is skipped. One emulator and
// one instance serve the whole test process; every
// test gets a fresh randomized database dropped on cleanup, so tests stay
// order-independent and parallelizable.
//
// Target extends that order with live Spanner — see its own documentation for
// the fixed database it uses and how to select it.
package conntest

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"terraform-provider-alis/internal/spanner/conn"

	databaseadmin "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	instanceadmin "cloud.google.com/go/spanner/admin/instance/apiv1"
	"cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"
	tcspanner "github.com/testcontainers/testcontainers-go/modules/gcloud/spanner"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	projectID  = "test-project"
	instanceID = "test-instance"

	defaultEmulatorImage = "gcr.io/cloud-spanner-emulator/emulator:latest"
)

var (
	setupOnce sync.Once
	setupErr  error
	dbCounter atomic.Int64
)

// ensureEmulator makes SPANNER_EMULATOR_HOST point at a running emulator with
// the test instance created, starting a container if needed. The container is
// shared for the process lifetime; testcontainers' reaper removes it afterwards.
func ensureEmulator() error {
	setupOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		if os.Getenv("SPANNER_EMULATOR_HOST") == "" {
			image := os.Getenv("SPANNER_EMULATOR_IMAGE")
			if image == "" {
				image = defaultEmulatorImage
			}
			container, err := tcspanner.Run(ctx, image, tcspanner.WithProjectID(projectID))
			if err != nil {
				setupErr = fmt.Errorf("no SPANNER_EMULATOR_HOST and could not start emulator container (is Docker running?): %w", err)
				return
			}
			if err := os.Setenv("SPANNER_EMULATOR_HOST", container.URI()); err != nil {
				setupErr = err
				return
			}
		}

		setupErr = ensureInstance(ctx)
	})
	return setupErr
}

// ensureInstance creates the shared test instance on the emulator, treating
// AlreadyExists as success so multiple test binaries can share one emulator.
func ensureInstance(ctx context.Context) error {
	ia, err := instanceadmin.NewInstanceAdminClient(ctx)
	if err != nil {
		return fmt.Errorf("instance admin client: %w", err)
	}
	defer ia.Close()

	op, err := ia.CreateInstance(ctx, &instancepb.CreateInstanceRequest{
		Parent:     "projects/" + projectID,
		InstanceId: instanceID,
		Instance: &instancepb.Instance{
			Config:      fmt.Sprintf("projects/%s/instanceConfigs/emulator-config", projectID),
			DisplayName: "conntest",
			NodeCount:   1,
		},
	})
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			return nil
		}
		return fmt.Errorf("create emulator instance: %w", err)
	}
	if _, err := op.Wait(ctx); err != nil && status.Code(err) != codes.AlreadyExists {
		return fmt.Errorf("wait for emulator instance: %w", err)
	}
	return nil
}

// TrySetup is Setup without the fatal path: it skips only when no emulator is
// reachable, and returns database-creation errors (e.g. an unsupported
// dialect) to the caller so support probes can report them.
func TrySetup(t *testing.T, dialect databasepb.DatabaseDialect) (conn.Connection, string, error) {
	t.Helper()

	if err := ensureEmulator(); err != nil {
		t.Skipf("spanner emulator unavailable: %v", err)
	}

	database, err := createDatabase(t, dialect)
	if err != nil {
		return nil, "", err
	}

	// Construct after SPANNER_EMULATOR_HOST is set — the adapter reads it at
	// construction time. Fast backoff: emulator retries need no politeness.
	cn := conn.New(conn.Options{Retry: conn.RetryPolicy{Attempts: 3, InitialBackoff: 200 * time.Millisecond}})
	t.Cleanup(func() { _ = cn.Close() })

	return cn, database, nil
}

// Setup returns a Connection backed by the emulator plus the full resource
// name of a fresh database that is dropped when the test finishes.
func Setup(t *testing.T, dialect databasepb.DatabaseDialect) (conn.Connection, string) {
	t.Helper()
	cn, database, err := TrySetup(t, dialect)
	if err != nil {
		t.Fatalf("conntest.Setup: %v", err)
	}
	return cn, database
}

// Target resolves the Spanner backend for integration tests that exercise
// full service lifecycles.
//
// Resolution order:
//
//  1. ALIS_OS_LIVE set to a true value — live Spanner, chosen explicitly.
//     Without this, a reachable emulator always wins, so a developer with
//     Docker running could never reach the live-only tests.
//  2. SPANNER_EMULATOR_HOST set — that emulator, with a fresh throwaway
//     database.
//  3. Docker available — a testcontainers-managed emulator, same fresh
//     database semantics. Together with (2) this keeps CI and automated
//     agents on the emulator by default.
//  4. ALIS_OS_PROJECT and ALIS_OS_INSTANCE set — the fallback for developers
//     who want a real-Spanner run when no emulator is available.
//  5. Otherwise the test skips.
//
// The live database is the one named by ALIS_OS_DATABASE (default "tf-test")
// on the ALIS_OS_INSTANCE instance. It must already exist and is never
// dropped; tests create and remove their own objects inside it.
//
// live reports whether the returned database is real Spanner, for fixtures
// only one backend can host (e.g. proto bundles the test provisions itself
// on the emulator, or IAM reads the emulator does not surface).
func Target(t *testing.T) (cn conn.Connection, database string, live bool) {
	t.Helper()

	preferLive, _ := strconv.ParseBool(os.Getenv("ALIS_OS_LIVE"))
	if preferLive {
		if cn, database, ok := liveTarget(t); ok {
			return cn, database, true
		}
		t.Fatal("ALIS_OS_LIVE is set but ALIS_OS_PROJECT/ALIS_OS_INSTANCE are not")
	}

	if err := ensureEmulator(); err == nil {
		cn, database = Setup(t, databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL)
		return cn, database, false
	}

	if cn, database, ok := liveTarget(t); ok {
		return cn, database, true
	}

	t.Skip(
		"no Spanner backend: set SPANNER_EMULATOR_HOST, start Docker, or set ALIS_OS_PROJECT/ALIS_OS_INSTANCE (add ALIS_OS_LIVE=1 to prefer live over a running emulator)",
	)
	return nil, "", false
}

// liveTarget builds a Connection to the developer's own Spanner instance,
// reporting false when it is not configured.
func liveTarget(t *testing.T) (conn.Connection, string, bool) {
	t.Helper()

	project, instance := os.Getenv("ALIS_OS_PROJECT"), os.Getenv("ALIS_OS_INSTANCE")
	if project == "" || instance == "" {
		return nil, "", false
	}

	dbID := os.Getenv("ALIS_OS_DATABASE")
	if dbID == "" {
		dbID = "tf-test"
	}

	cn := conn.New(conn.Options{})
	t.Cleanup(func() { _ = cn.Close() })

	return cn, fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, dbID), true
}

// createDatabase creates a database in the given dialect and registers a
// cleanup that drops it. The PID + per-process counter in the name keeps
// concurrent test binaries sharing one emulator from colliding.
func createDatabase(t *testing.T, dialect databasepb.DatabaseDialect) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	da, err := databaseadmin.NewDatabaseAdminClient(ctx)
	if err != nil {
		return "", fmt.Errorf("database admin client: %w", err)
	}
	defer da.Close()

	// The two-digit counter keeps the name compliant with the provider's
	// database-name validation, which requires two trailing alphanumerics.
	dbID := fmt.Sprintf("tftest-%d-%02d", os.Getpid()%10000, dbCounter.Add(1))
	stmt := fmt.Sprintf("CREATE DATABASE `%s`", dbID)
	if dialect == databasepb.DatabaseDialect_POSTGRESQL {
		stmt = fmt.Sprintf("CREATE DATABASE %q", dbID)
	}

	op, err := da.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:          fmt.Sprintf("projects/%s/instances/%s", projectID, instanceID),
		CreateStatement: stmt,
		DatabaseDialect: dialect,
	})
	if err != nil {
		return "", fmt.Errorf("create database: %w", err)
	}
	db, err := op.Wait(ctx)
	if err != nil {
		return "", fmt.Errorf("wait for database: %w", err)
	}

	name := db.GetName()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		da, err := databaseadmin.NewDatabaseAdminClient(ctx)
		if err != nil {
			return
		}
		defer da.Close()
		_ = da.DropDatabase(ctx, &databasepb.DropDatabaseRequest{Database: name})
	})

	return name, nil
}
