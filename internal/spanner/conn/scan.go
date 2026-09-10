package conn

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// rowSource is the slice of *sql.Rows that scanInto needs. Naming it lets the
// scanner be unit-tested against in-memory rows without a database driver.
type rowSource interface {
	Columns() ([]string, error)
	Next() bool
	Scan(dest ...any) error
	Err() error
}

// scanInto fills dest from rows and reports how many rows it consumed.
//
//	dest *[]T or *[]*T → every row; zero rows leaves an empty non-nil slice.
//	dest *T            → the first row only; zero rows leaves dest untouched.
//
// T must be a struct. Columns map to fields by the `db:"COLUMN"` tag, or by
// field name when untagged, compared case-insensitively because the
// PostgreSQL dialect lowercases information_schema column names. Columns with
// no matching field are drained and dropped, so SELECT * stays usable.
// Callers decide what zero rows means; scanInto only reports the count.
func scanInto(rows rowSource, dest any) (int, error) {
	dv := reflect.ValueOf(dest)
	if dv.Kind() != reflect.Pointer || dv.IsNil() {
		return 0, fmt.Errorf("conn: query dest must be a non-nil pointer, got %T", dest)
	}
	target := dv.Elem()

	// Resolve the struct type being scanned and whether we collect many.
	var (
		elemType reflect.Type // struct type of one row
		wantMany bool
		ptrElems bool // slice elements are *T rather than T
	)
	switch target.Kind() {
	case reflect.Slice:
		wantMany = true
		elemType = target.Type().Elem()
		if elemType.Kind() == reflect.Pointer {
			ptrElems = true
			elemType = elemType.Elem()
		}
	case reflect.Struct:
		elemType = target.Type()
	default:
		return 0, fmt.Errorf("conn: query dest must point to a struct or slice of structs, got %T", dest)
	}
	if elemType.Kind() != reflect.Struct {
		return 0, fmt.Errorf("conn: query dest must point to a struct or slice of structs, got %T", dest)
	}

	columns, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	fieldIndex := matchColumns(elemType, columns)

	if wantMany {
		target.Set(reflect.MakeSlice(target.Type(), 0, 0))
	}

	n := 0
	for rows.Next() {
		row := reflect.New(elemType).Elem()
		targets := make([]any, len(columns))
		var deferred []deferredField
		for i, fi := range fieldIndex {
			if fi < 0 {
				targets[i] = new(any) // drain unmatched column
				continue
			}
			field := row.Field(fi)
			if scansNullDirectly(field) {
				targets[i] = field.Addr().Interface()
				continue
			}
			// Plain value fields (string, int64, bool...) would fail on NULL.
			// Scan through a pointer instead: database/sql turns NULL into nil,
			// and we leave the field at its zero value. This matches the
			// leniency the row structs were written against.
			pp := reflect.New(reflect.PointerTo(field.Type()))
			targets[i] = pp.Interface()
			deferred = append(deferred, deferredField{field: field, ptr: pp})
		}
		if err := rows.Scan(targets...); err != nil {
			return n, err
		}
		for _, d := range deferred {
			if p := d.ptr.Elem(); !p.IsNil() {
				d.field.Set(p.Elem())
			}
		}
		n++

		if !wantMany {
			target.Set(row)
			break
		}
		if ptrElems {
			target.Set(reflect.Append(target, row.Addr()))
		} else {
			target.Set(reflect.Append(target, row))
		}
	}
	if err := rows.Err(); err != nil {
		return n, err
	}
	return n, nil
}

// matchColumns returns, for each column, the index of the exported struct
// field it maps to, or -1 when nothing matches.
func matchColumns(structType reflect.Type, columns []string) []int {
	byName := make(map[string]int, structType.NumField())
	for i := range structType.NumField() {
		f := structType.Field(i)
		if !f.IsExported() {
			continue
		}
		name := f.Tag.Get("db")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		byName[strings.ToLower(name)] = i
	}

	out := make([]int, len(columns))
	for i, c := range columns {
		if fi, ok := byName[strings.ToLower(c)]; ok {
			out[i] = fi
		} else {
			out[i] = -1
		}
	}
	return out
}

// deferredField is a plain value field scanned through a **T so NULL can be
// observed; the value is copied into field after Scan when non-nil.
type deferredField struct {
	field reflect.Value
	ptr   reflect.Value
}

var scannerType = reflect.TypeFor[sql.Scanner]()

// scansNullDirectly reports whether field can receive NULL on its own: it is
// a pointer, or it (or its address) implements sql.Scanner, like
// sql.NullString.
func scansNullDirectly(field reflect.Value) bool {
	t := field.Type()
	return t.Kind() == reflect.Pointer || t.Implements(scannerType) || reflect.PointerTo(t).Implements(scannerType)
}
