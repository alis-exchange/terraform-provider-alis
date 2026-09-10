package conn

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/spanner"
	spannerAdmin "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	spannerdriver "github.com/googleapis/go-sql-spanner"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// emulatorHostEnv names the emulator address adapters read at construction.
const emulatorHostEnv = "SPANNER_EMULATOR_HOST"

// New builds the production Connection: the GCP adapter wrapped in the
// uniform retry policy. It performs no I/O; clients are created lazily on
// first use and cached per database thereafter. Callers may therefore build
// one while holding a lock.
//
// The result depends on more than opts — see EmulatorHost.
func New(opts Options) Connection {
	return WithRetry(newGCPAdapter(opts), opts.Retry)
}

// EmulatorHost reports the emulator address the next New will capture, empty
// when talking to real Spanner. It is the one input to New that does not
// arrive through Options, so anything caching Connections must include it
// alongside the Options in its cache key: two adapters built from identical
// Options but different hosts talk to different backends.
func EmulatorHost() string {
	return os.Getenv(emulatorHostEnv)
}

// gcpConn is the production adapter: it implements Connection over the real
// Google clients — the database admin client for DDL and metadata, and
// database/sql (over go-sql-spanner) for INFORMATION_SCHEMA queries. The admin
// client is created once per adapter; SQL pools and dialect lookups are cached
// per database. SPANNER_EMULATOR_HOST is captured at construction time, so
// the adapter must be built after the variable is set.
type gcpConn struct {
	opts         Options
	emulatorHost string

	adminOnce sync.Once
	admin     *spannerAdmin.DatabaseAdminClient
	adminErr  error

	mu       sync.Mutex
	sessions map[string]*sql.DB
	dialects map[string]Dialect
}

var _ Connection = (*gcpConn)(nil)

func newGCPAdapter(opts Options) *gcpConn {
	return &gcpConn{
		opts:         opts,
		emulatorHost: EmulatorHost(),
		sessions:     map[string]*sql.DB{},
		dialects:     map[string]Dialect{},
	}
}

// splitDatabase validates and splits a full database resource name; short or
// malformed names return an error rather than panicking.
func splitDatabase(database string) (project, instance, db string, err error) {
	parts := strings.Split(database, "/")
	if len(parts) != 6 || parts[0] != "projects" || parts[2] != "instances" || parts[4] != "databases" {
		return "", "", "", status.Errorf(codes.InvalidArgument,
			"invalid database name %q, expected projects/{p}/instances/{i}/databases/{d}", database)
	}
	return parts[1], parts[3], parts[5], nil
}

// adminClient returns the shared database admin client, creating it on first
// use. A creation failure is sticky: the error is cached and returned to every
// subsequent caller for the adapter's lifetime.
func (g *gcpConn) adminClient(ctx context.Context) (*spannerAdmin.DatabaseAdminClient, error) {
	g.adminOnce.Do(func() {
		g.admin, g.adminErr = spannerAdmin.NewDatabaseAdminClient(ctx,
			clientOptions(g.opts.Credentials, g.emulatorHost)...)
	})
	return g.admin, g.adminErr
}

// session returns the cached SQL pool for a database, building it on first
// use from a go-sql-spanner connector that carries this Conn's credentials —
// a DSN-string sql.Open has no credential parameter, so the connector is the
// only route by which credentials can reach the query path.
func (g *gcpConn) session(database string) (*sql.DB, error) {
	g.mu.Lock()
	if db, ok := g.sessions[database]; ok {
		g.mu.Unlock()
		return db, nil
	}
	g.mu.Unlock()

	project, instance, dbID, err := splitDatabase(database)
	if err != nil {
		return nil, err
	}

	connector, err := spannerdriver.CreateConnector(spannerdriver.ConnectorConfig{
		Project:  project,
		Instance: instance,
		Database: dbID,
		Configurator: func(_ *spanner.ClientConfig, copts *[]option.ClientOption) {
			*copts = append(*copts, clientOptions(g.opts.Credentials, g.emulatorHost)...)
		},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating Spanner connector: %v", err)
	}

	db := sql.OpenDB(connector)

	g.mu.Lock()
	defer g.mu.Unlock()
	if cached, ok := g.sessions[database]; ok {
		_ = db.Close() // lost the race; keep the first pool
		return cached, nil
	}
	g.sessions[database] = db
	return db, nil
}

func (g *gcpConn) Dialect(ctx context.Context, database string) (Dialect, error) {
	g.mu.Lock()
	if d, ok := g.dialects[database]; ok {
		g.mu.Unlock()
		return d, nil
	}
	g.mu.Unlock()

	admin, err := g.adminClient(ctx)
	if err != nil {
		return DialectUnknown, err
	}
	db, err := admin.GetDatabase(ctx, &databasepb.GetDatabaseRequest{Name: database})
	if err != nil {
		return DialectUnknown, err
	}

	var d Dialect
	switch db.GetDatabaseDialect() {
	case databasepb.DatabaseDialect_POSTGRESQL:
		d = DialectPostgreSQL
	default:
		d = DialectGoogleSQL
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	g.dialects[database] = d
	return d, nil
}

func (g *gcpConn) ExecuteDDL(ctx context.Context, database string, statements ...string) error {
	return g.ExecuteDDLWithDescriptors(ctx, database, nil, statements...)
}

func (g *gcpConn) ExecuteDDLWithDescriptors(ctx context.Context, database string, protoDescriptors []byte, statements ...string) error {
	if len(statements) == 0 {
		return nil
	}
	admin, err := g.adminClient(ctx)
	if err != nil {
		return err
	}
	op, err := admin.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:         database,
		Statements:       statements,
		ProtoDescriptors: protoDescriptors,
	})
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

func (g *gcpConn) Query(ctx context.Context, database string, dest any, query string, params ...any) error {
	db, err := g.session(database)
	if err != nil {
		return err
	}

	start := time.Now()
	rows, err := db.QueryContext(ctx, query, params...)
	if err != nil {
		return err
	}
	defer rows.Close()

	n, err := scanInto(rows, dest)
	if err == nil {
		err = rows.Err() // scanInto already checks this; repeated for static analysis
	}
	tflog.Debug(ctx, "spanner query", map[string]any{
		"database":   database,
		"sql":        query,
		"rows":       n,
		"elapsed_ms": time.Since(start).Milliseconds(),
	})
	if err != nil {
		return err
	}
	// Port contract: dest *T with zero rows is codes.NotFound; dest *[]T
	// with zero rows is an empty slice and nil error.
	if n == 0 && !isSlicePointer(dest) {
		return status.Error(codes.NotFound, "no rows")
	}
	return nil
}

func isSlicePointer(dest any) bool {
	v := reflect.ValueOf(dest)
	return v.Kind() == reflect.Pointer && v.Elem().Kind() == reflect.Slice
}

func (g *gcpConn) DatabaseRoles(ctx context.Context, database string, pageSize int32, pageToken string) ([]string, string, error) {
	admin, err := g.adminClient(ctx)
	if err != nil {
		return nil, "", err
	}

	var names []string
	var nextPageToken string
	it := admin.ListDatabaseRoles(ctx, &databasepb.ListDatabaseRolesRequest{
		Parent:    database,
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	for {
		r, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		names = append(names, r.GetName())

		// The iterator streams across server pages, so enforce pageSize
		// manually and surface the token the caller needs to resume.
		if pageSize > 0 && len(names) >= int(pageSize) {
			nextPageToken = it.PageInfo().Token
			break
		}
	}

	return names, nextPageToken, nil
}

func (g *gcpConn) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for name, db := range g.sessions {
		_ = db.Close()
		delete(g.sessions, name)
	}
	if g.admin != nil {
		return g.admin.Close()
	}
	return nil
}
