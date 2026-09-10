package conn

import (
	"database/sql"
	"errors"
	"testing"
)

// fakeRows is an in-memory rowSource: columns plus a grid of values, each
// cell delivered to Scan the way database/sql delivers driver values.
type fakeRows struct {
	columns []string
	values  [][]any
	pos     int
	err     error
}

func (r *fakeRows) Columns() ([]string, error) { return r.columns, nil }

func (r *fakeRows) Next() bool {
	if r.pos >= len(r.values) {
		return false
	}
	r.pos++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	row := r.values[r.pos-1]
	for i, d := range dest {
		switch d := d.(type) {
		case *string:
			*d = row[i].(string)
		case *int64:
			*d = row[i].(int64)
		case **string:
			if row[i] == nil {
				*d = nil
			} else {
				s := row[i].(string)
				*d = &s
			}
		case **int64:
			if row[i] == nil {
				*d = nil
			} else {
				n := row[i].(int64)
				*d = &n
			}
		case *sql.NullString:
			if row[i] == nil {
				*d = sql.NullString{}
			} else {
				*d = sql.NullString{String: row[i].(string), Valid: true}
			}
		case *any:
			*d = row[i]
		default:
			return errors.New("fakeRows: unsupported scan target")
		}
	}
	return nil
}

func (r *fakeRows) Err() error { return r.err }

type taggedRow struct {
	Name   string  `db:"NAME"`
	Count  int64   `db:"COUNT"`
	Option *string `db:"OPTION_NAME"`
	Note   sql.NullString
	ignore string //nolint:unused // proves unexported fields are skipped
}

func TestScanInto_SliceOfStructs(t *testing.T) {
	rows := &fakeRows{
		columns: []string{"NAME", "COUNT", "OPTION_NAME", "NOTE"},
		values: [][]any{
			{"a", int64(1), "x", "n1"},
			{"b", int64(2), nil, nil},
		},
	}
	var got []taggedRow
	n, err := scanInto(rows, &got)
	if err != nil {
		t.Fatalf("scanInto: %v", err)
	}
	if n != 2 || len(got) != 2 {
		t.Fatalf("n = %d, len = %d, want 2", n, len(got))
	}
	if got[0].Name != "a" || got[0].Count != 1 || got[0].Option == nil || *got[0].Option != "x" || !got[0].Note.Valid {
		t.Fatalf("row 0 = %+v", got[0])
	}
	if got[1].Name != "b" || got[1].Option != nil || got[1].Note.Valid {
		t.Fatalf("row 1 = %+v", got[1])
	}
}

func TestScanInto_SliceOfPointers(t *testing.T) {
	rows := &fakeRows{
		columns: []string{"NAME", "COUNT"},
		values:  [][]any{{"a", int64(1)}},
	}
	var got []*taggedRow
	if _, err := scanInto(rows, &got); err != nil {
		t.Fatalf("scanInto: %v", err)
	}
	if len(got) != 1 || got[0].Name != "a" || got[0].Count != 1 {
		t.Fatalf("got = %+v", got)
	}
}

func TestScanInto_SingleStruct(t *testing.T) {
	rows := &fakeRows{
		columns: []string{"NAME", "COUNT"},
		values:  [][]any{{"first", int64(1)}, {"second", int64(2)}},
	}
	var got taggedRow
	n, err := scanInto(rows, &got)
	if err != nil {
		t.Fatalf("scanInto: %v", err)
	}
	if n != 1 || got.Name != "first" {
		t.Fatalf("n = %d, got = %+v, want first row only", n, got)
	}
}

func TestScanInto_ZeroRows(t *testing.T) {
	rows := &fakeRows{columns: []string{"NAME"}}

	var slice []taggedRow
	n, err := scanInto(rows, &slice)
	if err != nil || n != 0 {
		t.Fatalf("slice: n = %d, err = %v", n, err)
	}
	if slice == nil || len(slice) != 0 {
		t.Fatalf("slice = %#v, want empty non-nil", slice)
	}

	var single taggedRow
	n, err = scanInto(&fakeRows{columns: []string{"NAME"}}, &single)
	if err != nil || n != 0 {
		t.Fatalf("single: n = %d, err = %v", n, err)
	}
}

func TestScanInto_UnmatchedColumnsAreDiscarded(t *testing.T) {
	// SELECT * on INFORMATION_SCHEMA returns far more columns than the row
	// struct declares; they must be drained, not rejected.
	rows := &fakeRows{
		columns: []string{"TABLE_CATALOG", "NAME", "TABLE_SCHEMA", "COUNT"},
		values:  [][]any{{"", "a", "", int64(3)}},
	}
	var got taggedRow
	if _, err := scanInto(rows, &got); err != nil {
		t.Fatalf("scanInto: %v", err)
	}
	if got.Name != "a" || got.Count != 3 {
		t.Fatalf("got = %+v", got)
	}
}

func TestScanInto_ColumnMatchIsCaseInsensitive(t *testing.T) {
	// PostgreSQL-dialect databases return lowercase information_schema
	// column names for the same queries.
	rows := &fakeRows{
		columns: []string{"name", "count"},
		values:  [][]any{{"a", int64(1)}},
	}
	var got taggedRow
	if _, err := scanInto(rows, &got); err != nil {
		t.Fatalf("scanInto: %v", err)
	}
	if got.Name != "a" || got.Count != 1 {
		t.Fatalf("got = %+v", got)
	}
}

func TestScanInto_RowsErrIsReturned(t *testing.T) {
	want := errors.New("stream broke")
	rows := &fakeRows{columns: []string{"NAME"}, err: want}
	var got []taggedRow
	if _, err := scanInto(rows, &got); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestScanInto_RejectsBadDest(t *testing.T) {
	rows := &fakeRows{columns: []string{"NAME"}}
	for _, dest := range []any{nil, taggedRow{}, new(string), (*taggedRow)(nil), new([]string)} {
		if _, err := scanInto(rows, dest); err == nil {
			t.Errorf("dest %T: expected error", dest)
		}
	}
}

func TestScanInto_NullIntoPlainFieldIsZero(t *testing.T) {
	// LEFT JOIN rows can carry NULL into plain string/int fields. gorm mapped
	// those to the zero value; the scanner must keep that leniency.
	rows := &fakeRows{
		columns: []string{"NAME", "COUNT"},
		values:  [][]any{{nil, nil}},
	}
	var got taggedRow
	if _, err := scanInto(rows, &got); err != nil {
		t.Fatalf("scanInto: %v", err)
	}
	if got.Name != "" || got.Count != 0 {
		t.Fatalf("got = %+v, want zero values", got)
	}
}
