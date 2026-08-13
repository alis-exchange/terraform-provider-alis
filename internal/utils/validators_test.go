package utils

import (
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidateArgument(t *testing.T) {
	cases := map[string]struct {
		value string
		regex string
		want  bool
	}{
		"database name valid": {
			value: "projects/my-project/instances/my-instance/databases/my-db01",
			regex: SpannerGoogleSqlDatabaseNameRegex,
			want:  true,
		},
		"database name uppercase project": {
			value: "projects/My-Project/instances/my-instance/databases/my-db01",
			regex: SpannerGoogleSqlDatabaseNameRegex,
			want:  false,
		},
		"database name wrong collection": {
			value: "projects/my-project/instances/my-instance/tables/my-db01",
			regex: SpannerGoogleSqlDatabaseNameRegex,
			want:  false,
		},
		"table name valid": {
			value: "projects/my-project/instances/my-instance/databases/my-db01/tables/MyTable_1",
			regex: SpannerGoogleSqlTableNameRegex,
			want:  true,
		},
		"table name with dash": {
			value: "projects/my-project/instances/my-instance/databases/my-db01/tables/my-table",
			regex: SpannerGoogleSqlTableNameRegex,
			want:  false,
		},
		"table id valid": {
			value: "MyTable_1",
			regex: SpannerGoogleSqlTableIdRegex,
			want:  true,
		},
		"table id leading digit": {
			value: "1table",
			regex: SpannerGoogleSqlTableIdRegex,
			want:  false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := ValidateArgument(tc.value, tc.regex); got != tc.want {
				t.Errorf("ValidateArgument(%q) = %t, want %t", tc.value, got, tc.want)
			}
		})
	}
}

func TestCutPrefixAndSuffix(t *testing.T) {
	cases := map[string]struct {
		s, prefix, suffix, want string
	}{
		"both present":   {`^[a-z]+$`, "^", "$", `[a-z]+`},
		"none present":   {"abc", "^", "$", "abc"},
		"prefix only":    {"^abc", "^", "$", "abc"},
		"suffix only":    {"abc$", "^", "$", "abc"},
		"empty string":   {"", "^", "$", ""},
		"empty affixes":  {"abc", "", "", "abc"},
		"suffix earlier": {"$abc", "^", "$", "$abc"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := CutPrefixAndSuffix(tc.s, tc.prefix, tc.suffix); got != tc.want {
				t.Errorf("CutPrefixAndSuffix(%q, %q, %q) = %q, want %q", tc.s, tc.prefix, tc.suffix, got, tc.want)
			}
		})
	}
}

// The helper's error message must quote the same patterns it tested against.
// Every services entry point validates through it, and the hand-written blocks
// it replaced had drifted into citing rules they never applied.
func TestValidateDialectArgument(t *testing.T) {
	cases := map[string]struct {
		field, value           string
		googleSql, postgresSql string
		wantErr                bool
	}{
		"matches googlesql pattern": {
			field: "name", value: "tftest_table",
			googleSql: SpannerGoogleSqlTableIdRegex, postgresSql: SpannerPostgresSqlTableIdRegex,
		},
		"matches postgres pattern only": {
			field: "database", value: "projects/my-project/instances/my-instance/databases/MyDb01",
			googleSql: SpannerGoogleSqlDatabaseNameRegex, postgresSql: SpannerPostgresSqlDatabaseNameRegex,
		},
		"matches neither": {
			field: "role", value: "admin1, admin2",
			googleSql: SpannerGoogleSqlRoleIdRegex, postgresSql: SpannerPostgresSqlRoleIdRegex,
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateDialectArgument(tc.field, tc.value, tc.googleSql, tc.postgresSql)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("ValidateDialectArgument(%q, %q, ...) = %v, want nil", tc.field, tc.value, err)
				}
				return
			}

			if err == nil {
				t.Fatalf("ValidateDialectArgument(%q, %q, ...) = nil, want an error", tc.field, tc.value)
			}
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("code = %v, want InvalidArgument", status.Code(err))
			}
			for _, want := range []string{tc.field, tc.value, tc.googleSql, tc.postgresSql} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

// Pattern is what makes validation compile-once; a fresh compile per call
// would be silently correct, so assert on identity.
func TestPatternCompilesOnce(t *testing.T) {
	first := Pattern(SpannerGoogleSqlTableIdRegex)
	second := Pattern(SpannerGoogleSqlTableIdRegex)
	if first != second {
		t.Error("Pattern returned distinct compilations for the same expression")
	}
	if !first.MatchString("tftest_table") {
		t.Errorf("compiled pattern %v does not match a valid table ID", first)
	}
}
