// Package acctest wires terraform-plugin-testing acceptance tests to a real
// Spanner backend. Backend resolution reuses conntest.Target: a running
// emulator via SPANNER_EMULATOR_HOST, a Docker emulator via testcontainers,
// live Spanner via ALIS_OS_PROJECT/ALIS_OS_INSTANCE (set ALIS_OS_LIVE=1 to
// choose it over a reachable emulator), or skip.
//
// On the emulator each test gets a fresh database, dropped on cleanup after
// the test's own destroy has run. Live runs share one long-lived database, so
// tests there must name their objects distinctly and clean up after
// themselves.
package acctest

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"terraform-provider-alis/internal/provider"
	"terraform-provider-alis/internal/spanner/conn"
	"terraform-provider-alis/internal/spanner/conn/conntest"
	"terraform-provider-alis/internal/spanner/names"
	"terraform-provider-alis/internal/spanner/services"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProtoV6ProviderFactories returns the factory map for resource.TestCase. The
// key must be the provider type name ("alis" — what configs reference), not
// the registry address.
func ProtoV6ProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"alis": providerserver.NewProtocol6WithError(provider.NewProvider("test")()),
	}
}

// Env describes the Spanner backend a single acceptance test runs against.
type Env struct {
	// Project, Instance, and Database are the parsed IDs, ready for
	// interpolation into test configs.
	Project  string
	Instance string
	Database string
	// DatabaseName is the full resource name
	// "projects/{p}/instances/{i}/databases/{d}".
	DatabaseName string
	// Live reports whether the backend is real Spanner rather than the
	// emulator. Tests needing APIs the emulator lacks gate on it.
	Live bool
	// Conn and Service talk to the same backend the provider under test
	// will, for probes and CheckDestroy assertions.
	Conn    conn.Connection
	Service *services.SpannerService
}

// Setup resolves a backend and creates a fresh database for one acceptance
// test. Cleanup ordering matters: conntest registers the database drop before
// resource.Test runs, so the test's own destroy executes first.
func Setup(t *testing.T) Env {
	t.Helper()

	// Skip before any backend work: resource.Test would skip on its own, but
	// only after this helper had already booted a Docker emulator.
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	cn, database, live := conntest.Target(t)
	dbName, err := names.ParseDatabase(database)
	if err != nil {
		t.Fatalf("parse database name %q: %v", database, err)
	}

	return Env{
		Project:      dbName.Project,
		Instance:     dbName.Instance,
		Database:     dbName.Database,
		DatabaseName: database,
		Live:         live,
		Conn:         cn,
		Service:      services.NewSpannerService(cn),
	}
}

// ProviderBlock renders the provider configuration for this backend. Against
// the emulator a static access token plus project keeps Configure fully
// hermetic: utils.GetGoogleCredentials builds a static token source without
// touching ADC, and conn suppresses credentials entirely while
// SPANNER_EMULATOR_HOST is set. Against live Spanner the empty block falls
// through to Application Default Credentials.
func (e Env) ProviderBlock() string {
	if e.Live {
		return "provider \"alis\" {}\n"
	}
	return fmt.Sprintf(`
provider "alis" {
  access_token = "emulator-placeholder-token"
  project      = %q
}
`, e.Project)
}

// SkipIfNoRoleListing skips the test when the backend cannot list database
// roles: the emulator does not implement the admin API at all, and a live
// principal may lack the permission or be unreachable. The probe keeps the
// gate behavioral — the test starts running the moment the capability
// appears — and it never fails the build for an environmental condition.
//
// The probe deadline is short and deliberate: the live Connection retries for
// over a minute by default, which would turn an unauthorized principal into a
// long stall before the skip.
func (e Env) SkipIfNoRoleListing(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	_, _, err := e.Conn.DatabaseRoles(ctx, e.DatabaseName, 1, "")
	if err != nil {
		t.Skipf("backend cannot list database roles (%v); set ALIS_OS_PROJECT/ALIS_OS_INSTANCE with ALIS_OS_LIVE=1 for live coverage", err)
	}
}

// CheckNotFound adapts a lookup into a check that passes only when the object
// is gone. Use it for CheckDestroy, and for the step that destroys a child
// resource on its own: when the parent table is dropped in the same destroy,
// every child lookup reports NotFound whether or not the provider issued its
// own DROP, so CheckDestroy alone cannot tell a working Delete from a no-op.
func CheckNotFound(kind, id string, lookup func() error) func(*terraform.State) error {
	return func(*terraform.State) error {
		err := lookup()
		if status.Code(err) == codes.NotFound {
			return nil
		}
		if err != nil {
			return fmt.Errorf("checking %s %q: %w", kind, id, err)
		}

		return fmt.Errorf("%s %q still exists", kind, id)
	}
}

// SkipIfNotLive skips the test unless it runs against real Spanner. Use it
// for behavior the emulator cannot exercise at all, with a reason naming the
// missing capability.
func (e Env) SkipIfNotLive(t *testing.T, reason string) {
	t.Helper()

	if !e.Live {
		t.Skip(reason)
	}
}
