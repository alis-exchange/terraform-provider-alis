// Package connfake is the in-memory adapter for the Connection port. It
// records every call in order, serves canned query results, and injects
// errors — the test surface for call choreography, retry policy, and
// schema-drift scenarios.
package connfake

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"terraform-provider-alis/internal/spanner/conn"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"context"
)

type OpKind string

const (
	OpDialect       OpKind = "Dialect"
	OpExecuteDDL    OpKind = "ExecuteDDL"
	OpExec          OpKind = "Exec"
	OpQuery         OpKind = "Query"
	OpDatabaseRoles OpKind = "DatabaseRoles"
)

// Op is one recorded call, in arrival order.
type Op struct {
	Kind             OpKind
	Database         string
	Statements       []string // ExecuteDDL / ExecuteDDLWithDescriptors
	ProtoDescriptors []byte   // ExecuteDDLWithDescriptors
	SQL              string   // Exec / Query
	Params           []any
}

type queryStub struct {
	pred func(Op) bool
	fill func(dest any) error
}

type failure struct {
	n   int
	err error
}

// Fake implements conn.Connection.
type Fake struct {
	mu       sync.Mutex
	ops      []Op
	dialects map[string]conn.Dialect
	roles    map[string][]string
	stubs    []queryStub // matched most-recently-registered first
	failures map[OpKind]*failure
}

var _ conn.Connection = (*Fake)(nil)

func New() *Fake {
	return &Fake{
		dialects: map[string]conn.Dialect{},
		roles:    map[string][]string{},
		failures: map[OpKind]*failure{},
	}
}

// SetDatabaseRoles seeds the role resource names returned for a database.
func (f *Fake) SetDatabaseRoles(database string, names []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.roles[database] = names
}

// --- Seeding ---

func (f *Fake) SetDialect(database string, d conn.Dialect) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dialects[database] = d
}

// OnQuery serves canned rows for any Query whose SQL contains sqlContains.
// rows must be a slice whose type matches the caller's dest (e.g.
// []*services.SequenceRow for dest *[]*services.SequenceRow, or the element
// type for dest *T); a mismatch fails the Query loudly.
func (f *Fake) OnQuery(sqlContains string, rows any) {
	f.OnQueryFunc(
		func(op Op) bool { return strings.Contains(op.SQL, sqlContains) },
		func(dest any) error { return fillDest(dest, rows) },
	)
}

// OnQueryFunc is the fully general form for drift scenarios: pred picks the
// call, fill populates dest.
func (f *Fake) OnQueryFunc(pred func(Op) bool, fill func(dest any) error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stubs = append(f.stubs, queryStub{pred: pred, fill: fill})
}

// --- Error injection ---

// FailNext makes the next n ops of kind fail with err, then recover.
func (f *Fake) FailNext(kind OpKind, n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures[kind] = &failure{n: n, err: err}
}

// --- Recording & assertions ---

func (f *Fake) Ops() []Op {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Op, len(f.ops))
	copy(out, f.ops)
	return out
}

func (f *Fake) OpsOf(kind OpKind) []Op {
	var out []Op
	for _, op := range f.Ops() {
		if op.Kind == kind {
			out = append(out, op)
		}
	}
	return out
}

// Statements returns every DDL statement and Exec SQL, flattened, in call order.
func (f *Fake) Statements() []string {
	var out []string
	for _, op := range f.Ops() {
		switch op.Kind {
		case OpExecuteDDL:
			out = append(out, op.Statements...)
		case OpExec:
			out = append(out, op.SQL)
		}
	}
	return out
}

// AssertSubsequence fails t unless wants appear as an in-order subsequence
// (substring match) of Statements().
func (f *Fake) AssertSubsequence(t testing.TB, wants ...string) {
	t.Helper()
	stmts := f.Statements()
	i := 0
	for _, want := range wants {
		found := false
		for ; i < len(stmts); i++ {
			if strings.Contains(stmts[i], want) {
				found = true
				i++
				break
			}
		}
		if !found {
			t.Errorf("statement %q not found in order within %q", want, stmts)
			return
		}
	}
}

// --- conn.Connection implementation ---

func (f *Fake) record(op Op) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ops = append(f.ops, op)
	if fail, ok := f.failures[op.Kind]; ok && fail.n > 0 {
		fail.n--
		return fail.err
	}
	return nil
}

func (f *Fake) Dialect(_ context.Context, database string) (conn.Dialect, error) {
	if err := f.record(Op{Kind: OpDialect, Database: database}); err != nil {
		return conn.DialectUnknown, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if d, ok := f.dialects[database]; ok {
		return d, nil
	}
	return conn.DialectGoogleSQL, nil
}

func (f *Fake) ExecuteDDL(_ context.Context, database string, statements ...string) error {
	return f.record(Op{Kind: OpExecuteDDL, Database: database, Statements: statements})
}

func (f *Fake) ExecuteDDLWithDescriptors(_ context.Context, database string, protoDescriptors []byte, statements ...string) error {
	return f.record(Op{Kind: OpExecuteDDL, Database: database, Statements: statements, ProtoDescriptors: protoDescriptors})
}

func (f *Fake) Exec(_ context.Context, database string, sql string, params ...any) error {
	return f.record(Op{Kind: OpExec, Database: database, SQL: sql, Params: params})
}

func (f *Fake) Query(_ context.Context, database string, dest any, sql string, params ...any) error {
	op := Op{Kind: OpQuery, Database: database, SQL: sql, Params: params}
	if err := f.record(op); err != nil {
		return err
	}

	f.mu.Lock()
	stubs := make([]queryStub, len(f.stubs))
	copy(stubs, f.stubs)
	f.mu.Unlock()

	for i := len(stubs) - 1; i >= 0; i-- {
		if stubs[i].pred(op) {
			return stubs[i].fill(dest)
		}
	}
	// Unstubbed query follows the port contract for zero rows.
	return fillDest(dest, nil)
}

func (f *Fake) DatabaseRoles(_ context.Context, database string, pageSize int32, _ string) ([]string, string, error) {
	if err := f.record(Op{Kind: OpDatabaseRoles, Database: database}); err != nil {
		return nil, "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	names := f.roles[database]
	if pageSize > 0 && int(pageSize) < len(names) {
		return names[:pageSize], "next", nil
	}
	return names, "", nil
}

func (f *Fake) Close() error { return nil }

// fillDest implements the port's scan contract: dest *[]T gets all rows
// (empty slice for none), dest *T gets the first row or codes.NotFound.
func fillDest(dest any, rows any) error {
	dv := reflect.ValueOf(dest)
	if dv.Kind() != reflect.Pointer || dv.IsNil() {
		return fmt.Errorf("connfake: dest must be a non-nil pointer, got %T", dest)
	}
	elem := dv.Elem()

	var rv reflect.Value
	if rows != nil {
		rv = reflect.ValueOf(rows)
		if rv.Kind() != reflect.Slice {
			return fmt.Errorf("connfake: seeded rows must be a slice, got %T", rows)
		}
	}

	if elem.Kind() == reflect.Slice {
		if rows == nil {
			elem.Set(reflect.MakeSlice(elem.Type(), 0, 0))
			return nil
		}
		if !rv.Type().AssignableTo(elem.Type()) {
			return fmt.Errorf("connfake: seeded rows %T not assignable to dest %T", rows, dest)
		}
		elem.Set(rv)
		return nil
	}

	// dest *T → first row or NotFound.
	if rows == nil || rv.Len() == 0 {
		return status.Error(codes.NotFound, "connfake: no rows for single-row dest")
	}
	first := rv.Index(0)
	if first.Kind() == reflect.Pointer && first.Type().Elem() == elem.Type() {
		elem.Set(first.Elem())
		return nil
	}
	if first.Type() == elem.Type() {
		elem.Set(first)
		return nil
	}
	return fmt.Errorf("connfake: seeded row %s not assignable to dest %T", first.Type(), dest)
}
