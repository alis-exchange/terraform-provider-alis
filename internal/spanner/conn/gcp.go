package conn

import (
	"context"
	"errors"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/spanner"
	spannerAdmin "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	spannergorm "github.com/googleapis/go-gorm-spanner"
	spannerdriver "github.com/googleapis/go-sql-spanner"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	customloggers "terraform-provider-alis/internal/spanner/logger"
)

// New builds the production Connection: the GCP adapter wrapped in the
// uniform retry policy. It performs no I/O; clients are created lazily on
// first use and cached per database thereafter.
func New(opts Options) Connection {
	return WithRetry(newGCPAdapter(opts), opts.Retry)
}

// defaultGormLogger is the single logger configuration applied to every
// session the adapter hands out.
func defaultGormLogger() gormlogger.Interface {
	return customloggers.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormlogger.Info,
			IgnoreRecordNotFoundError: false,
			ParameterizedQueries:      true,
			Colorful:                  true,
		},
	)
}

type gcpConn struct {
	opts         Options
	emulatorHost string
	logger       gormlogger.Interface

	adminOnce sync.Once
	admin     *spannerAdmin.DatabaseAdminClient
	adminErr  error

	mu       sync.Mutex
	sessions map[string]*gorm.DB
	dialects map[string]Dialect
}

var (
	_ Connection = (*gcpConn)(nil)
	_ MetadataDB = (*gcpConn)(nil)
)

func newGCPAdapter(opts Options) *gcpConn {
	return &gcpConn{
		opts:         opts,
		emulatorHost: os.Getenv("SPANNER_EMULATOR_HOST"),
		logger:       defaultGormLogger(),
		sessions:     map[string]*gorm.DB{},
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

func (g *gcpConn) adminClient(ctx context.Context) (*spannerAdmin.DatabaseAdminClient, error) {
	g.adminOnce.Do(func() {
		g.admin, g.adminErr = spannerAdmin.NewDatabaseAdminClient(ctx,
			clientOptions(g.opts.Credentials, g.emulatorHost)...)
	})
	return g.admin, g.adminErr
}

// session returns the cached gorm handle for a database, building it on first
// use from a go-sql-spanner connector that carries this Conn's credentials —
// a DSN-string gorm.Open has no credential parameter, so the connector is the
// only route by which credentials can reach the gorm path.
func (g *gcpConn) session(ctx context.Context, database string) (*gorm.DB, error) {
	g.mu.Lock()
	if db, ok := g.sessions[database]; ok {
		g.mu.Unlock()
		return db.WithContext(ctx), nil
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

	db, err := gorm.Open(
		spannergorm.New(spannergorm.Config{Connector: connector}),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      g.logger,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error connecting to database: %v", err)
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if cached, ok := g.sessions[database]; ok {
		return cached.WithContext(ctx), nil
	}
	g.sessions[database] = db
	return db.WithContext(ctx), nil
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

func (g *gcpConn) Exec(ctx context.Context, database string, sql string, params ...any) error {
	db, err := g.session(ctx, database)
	if err != nil {
		return err
	}
	return db.Exec(sql, params...).Error
}

func (g *gcpConn) Query(ctx context.Context, database string, dest any, sql string, params ...any) error {
	db, err := g.session(ctx, database)
	if err != nil {
		return err
	}
	result := db.Raw(sql, params...).Scan(dest)
	if result.Error != nil {
		return result.Error
	}
	// Port contract: dest *T with zero rows is codes.NotFound; dest *[]T
	// with zero rows is an empty slice and nil error.
	if result.RowsAffected == 0 && !isSlicePointer(dest) {
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

		// Check if page size is reached
		if pageSize > 0 && len(names) >= int(pageSize) {
			nextPageToken = it.PageInfo().Token
			break
		}
	}

	return names, nextPageToken, nil
}

func (g *gcpConn) GormDB(ctx context.Context, database string) (*gorm.DB, error) {
	return g.session(ctx, database)
}

func (g *gcpConn) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for name, db := range g.sessions {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		delete(g.sessions, name)
	}
	if g.admin != nil {
		return g.admin.Close()
	}
	return nil
}
