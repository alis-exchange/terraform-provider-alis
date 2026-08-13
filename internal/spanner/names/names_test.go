package names

import (
	"errors"
	"testing"
)

func TestParseAndFormatRoundTrip(t *testing.T) {
	// Every shape must round-trip byte-identically: Parse(x).String() == x.
	cases := []struct {
		name  string
		parse func(string) (interface{ String() string }, error)
		input string
	}{
		{
			"database", func(s string) (interface{ String() string }, error) { n, err := ParseDatabase(s); return n, err },
			"projects/my-project/instances/my-instance/databases/my-db",
		},
		{
			"table", func(s string) (interface{ String() string }, error) { n, err := ParseTable(s); return n, err },
			"projects/my-project/instances/my-instance/databases/my-db/tables/my_table",
		},
		{
			"sequence", func(s string) (interface{ String() string }, error) { n, err := ParseSequence(s); return n, err },
			"projects/my-project/instances/my-instance/databases/my-db/sequences/my_sequence",
		},
		{
			"database role", func(s string) (interface{ String() string }, error) { n, err := ParseDatabaseRole(s); return n, err },
			"projects/my-project/instances/my-instance/databases/my-db/databaseRoles/my_role",
		},
		{
			"index", func(s string) (interface{ String() string }, error) { n, err := ParseIndex(s); return n, err },
			"projects/my-project/instances/my-instance/databases/my-db/tables/my_table/indexes/my_idx",
		},
		{
			"table role", func(s string) (interface{ String() string }, error) { n, err := ParseTableRole(s); return n, err },
			"projects/my-project/instances/my-instance/databases/my-db/tables/my_table/tableRoles/my_role",
		},
		{
			"foreign key", func(s string) (interface{ String() string }, error) { n, err := ParseForeignKey(s); return n, err },
			"projects/my-project/instances/my-instance/databases/my-db/tables/my_table/constraints/FK_my",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := tc.parse(tc.input)
			if err != nil {
				t.Fatalf("parse(%q): %v", tc.input, err)
			}
			if got := n.String(); got != tc.input {
				t.Errorf("round-trip drift: %q -> %q", tc.input, got)
			}
		})
	}
}

func TestParseRejectsMalformedNames(t *testing.T) {
	// Short, empty, and wrong-collection inputs return typed errors — never
	// panics, never silent misparses.
	bad := []string{
		"",
		"projects/p",
		"projects/p/instances/i",
		"projects/p/instances/i/databases",                  // odd segment count
		"project/p/instances/i/databases/d",                 // wrong keyword
		"projects/p/instance/i/databases/d",                 // wrong keyword
		"projects/p/instances/i/database/d",                 // wrong keyword
		"projects/p/instances/i/databases/d/extra",          // trailing segment
		"projects/p/instances/i/databases/d/tables/t/extra", // odd tail
	}

	for _, input := range bad {
		if _, err := ParseDatabase(
			input,
		); input == "projects/p/instances/i/databases/d/extra" ||
			input == "projects/p/instances/i/databases/d/tables/t/extra" {
			if err == nil {
				t.Errorf("ParseDatabase(%q) accepted a longer name", input)
			}
		} else if err == nil {
			t.Errorf("ParseDatabase(%q) = nil error, want ErrInvalidName", input)
		}
	}

	if _, err := ParseTable("projects/p/instances/i/databases/d"); err == nil {
		t.Error("ParseTable accepted a database name")
	}
	if _, err := ParseTable("projects/p/instances/i/databases/d/sequences/s"); err == nil {
		t.Error("ParseTable accepted a sequence name (wrong collection keyword)")
	}
	if _, err := ParseSequence("projects/p/instances/i/databases/d/tables/t"); err == nil {
		t.Error("ParseSequence accepted a table name")
	}

	// Errors are matchable via the sentinel.
	_, err := ParseTable("nope")
	if !errors.Is(err, ErrInvalidName) {
		t.Errorf("err = %v, want errors.Is(_, ErrInvalidName)", err)
	}
}

func TestComponentAccess(t *testing.T) {
	table, err := ParseTable("projects/p/instances/i/databases/d/tables/t")
	if err != nil {
		t.Fatal(err)
	}
	if table.Project != "p" || table.Instance != "i" || table.Database != "d" || table.Table != "t" {
		t.Errorf("components = %+v", table)
	}
	// The parent database name is directly available for the common
	// "split table name, rebuild database name" pattern in services.
	if got := table.DatabaseName().String(); got != "projects/p/instances/i/databases/d" {
		t.Errorf("DatabaseName() = %q", got)
	}

	idx, err := ParseIndex("projects/p/instances/i/databases/d/tables/t/indexes/x")
	if err != nil {
		t.Fatal(err)
	}
	if idx.Index != "x" || idx.TableName().String() != "projects/p/instances/i/databases/d/tables/t" {
		t.Errorf("index components = %+v", idx)
	}
}

func TestFormatFromComponents(t *testing.T) {
	db := DatabaseName{Project: "p", Instance: "i", Database: "d"}
	if got := db.String(); got != "projects/p/instances/i/databases/d" {
		t.Errorf("DatabaseName.String() = %q", got)
	}
	tbl := TableName{Project: "p", Instance: "i", Database: "d", Table: "t"}
	if got := tbl.String(); got != "projects/p/instances/i/databases/d/tables/t" {
		t.Errorf("TableName.String() = %q", got)
	}
}
