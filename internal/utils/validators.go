package utils

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Regex for project and instance.
var (
	ProjectIdRegex  = `^[a-z](?:[-a-z0-9]{4,28}[a-z0-9])?$`
	InstanceIdRegex = `^[a-z0-9-]{6,33}$`
)

// Bigtable regex.
var (
	InstanceNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
	)
	BigtableTableIdRegex   = `^[a-zA-Z0-9_.-]{1,50}$`
	BigtableTableNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/tables\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(BigtableTableIdRegex, "^", "$"),
	)
	BigtableColumnFamilyIdRegex = `^[-_.a-zA-Z0-9]{1,50}$`
	BigtableClusterIdRegex      = `^[a-z0-9-]{6,30}$`
	BigtableClusterNameRegex    = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/clusters\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(BigtableClusterIdRegex, "^", "$"),
	)
	BigtableBackupIdRegex   = `^[a-zA-Z0-9_.-]{1,50}$`
	BigtableBackupNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/clusters\/%s\/backups\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(BigtableClusterIdRegex, "^", "$"),
		CutPrefixAndSuffix(BigtableBackupIdRegex, "^", "$"),
	)
)

// Spanner regex.
var (
	SpannerGoogleSqlDatabaseIdRegex   = `^[a-z][a-z0-9_\-]*[a-z0-9]{2,30}$`
	SpannerPostgresSqlDatabaseIdRegex = `^[a-zA-Z][a-zA-Z0-9_]{2,30}$`
	SpannerGoogleSqlDatabaseNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlDatabaseIdRegex, "^", "$"),
	)
	SpannerPostgresSqlDatabaseNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlDatabaseIdRegex, "^", "$"),
	)
	// Role IDs name both database roles and the grantees of table
	// privileges, and reach DDL by concatenation — validate every one of them
	// against these before rendering GRANT/REVOKE/CREATE ROLE.
	SpannerGoogleSqlRoleIdRegex           = `^[a-zA-Z0-9_]{1,64}$`
	SpannerPostgresSqlRoleIdRegex         = `^[a-zA-Z0-9_]{1,64}$`
	SpannerGoogleSqlDatabaseRoleNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/databaseRoles\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlDatabaseIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlRoleIdRegex, "^", "$"),
	)
	SpannerPostgresSqlDatabaseRoleNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/databaseRoles\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlDatabaseIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlRoleIdRegex, "^", "$"),
	)
	SpannerGoogleSqlTableIdRegex   = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerPostgresSqlTableIdRegex = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerGoogleSqlTableNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlDatabaseIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlTableIdRegex, "^", "$"),
	)
	SpannerPostgresSqlTableNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlDatabaseIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlTableIdRegex, "^", "$"),
	)
	SpannerGoogleSqlTableRoleNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s\/tableRoles\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlDatabaseIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlTableIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlRoleIdRegex, "^", "$"),
	)
	SpannerPostgresSqlTableRoleNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s\/tableRoles\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlDatabaseIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlTableIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlRoleIdRegex, "^", "$"),
	)
	SpannerGoogleSqlBackupIdRegex   = `^[a-z][a-z0-9_\-]*[a-z0-9]{2,30}$`
	SpannerPostgresSqlBackupIdRegex = `^[a-zA-Z][a-zA-Z0-9_]{2,30}$`
	SpannerGoogleSqlBackupNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/backups\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlBackupIdRegex, "^", "$"),
	)
	SpannerPostgresSqlBackupNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/backups\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlBackupIdRegex, "^", "$"),
	)
	SpannerGoogleSqlColumnIdRegex       = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerPostgresSqlColumnIdRegex     = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerGoogleSqlIndexIdRegex        = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerPostgresSqlIndexIdRegex      = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerGoogleSqlTableIndexNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s\/indexes\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlDatabaseIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlTableIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlIndexIdRegex, "^", "$"),
	)
	SpannerPostgresSqlTableIndexNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s\/indexes\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlDatabaseIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlTableIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlIndexIdRegex, "^", "$"),
	)

	SpannerGoogleSqlConstraintIdRegex   = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerPostgresSqlConstraintIdRegex = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`

	SpannerGoogleSqlSequenceIdRegex   = `^[a-zA-Z0-9_]{1,64}$`
	SpannerPostgresSqlSequenceIdRegex = `^[a-zA-Z0-9_]{1,64}$`

	SpannerGoogleSqlSequenceNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/sequences\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlDatabaseIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSqlSequenceIdRegex, "^", "$"),
	)
	SpannerPostgresSqlSequenceNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/sequences\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(InstanceIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlDatabaseIdRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSqlSequenceIdRegex, "^", "$"),
	)
)

// Discovery Engine regex.
var (
	DiscoveryEngineDatastoreNameRegex = fmt.Sprintf(
		`^projects\/%s\/locations\/[a-zA-Z0-9-]*\/collections\/[a-zA-Z0-9-_]*\/dataStores\/[a-z0-9-_]*$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
	)
	DiscoveryEngineDatastoreSchemaIdRegex   = `^[a-zA-Z0-9-_]*$`
	DiscoveryEngineDatastoreSchemaNameRegex = fmt.Sprintf(
		`^projects\/%s\/locations\/[a-zA-Z0-9-]*\/collections\/[a-zA-Z0-9-_]*\/dataStores\/[a-z0-9-_]*\/schemas\/%s$`,
		CutPrefixAndSuffix(ProjectIdRegex, "^", "$"),
		CutPrefixAndSuffix(DiscoveryEngineDatastoreSchemaIdRegex, "^", "$"),
	)
)

// ValidateArgument reports whether value matches the given regular expression.
// The pattern is compiled with regexp.MustCompile, so an invalid pattern
// panics; callers pass the pre-defined patterns declared in this package.
func ValidateArgument(value, regex string) bool {
	return Pattern(regex).MatchString(value)
}

// ValidateDialectArgument returns an InvalidArgument error unless value
// matches the GoogleSQL or the PostgreSQL pattern for field. The message
// quotes the same two patterns the value was tested against, so it can never
// cite a rule that was not applied.
func ValidateDialectArgument(field, value, googleSqlRegex, postgresSqlRegex string) error {
	if ValidateArgument(value, googleSqlRegex) || ValidateArgument(value, postgresSqlRegex) {
		return nil
	}

	return status.Errorf(
		codes.InvalidArgument,
		"Invalid argument %s (%s), must match `%s` for GoogleSql dialect or `%s` for PostgreSQL dialect",
		field,
		value,
		googleSqlRegex,
		postgresSqlRegex,
	)
}

// patterns memoizes compiled expressions. Validation runs on every plan and
// refresh against a fixed set of long patterns, so compiling per call is pure
// repeated work.
var patterns sync.Map

// Pattern returns the compiled form of regex, compiling it at most once per
// distinct pattern. It panics on an invalid pattern, like regexp.MustCompile.
func Pattern(regex string) *regexp.Regexp {
	if cached, ok := patterns.Load(regex); ok {
		if compiled, ok := cached.(*regexp.Regexp); ok {
			return compiled
		}
	}

	compiled := regexp.MustCompile(regex)
	patterns.Store(regex, compiled)

	return compiled
}

// CutPrefixAndSuffix cuts the prefix and suffix from a string
// If the prefix or suffix is not present, the string is returned as is.
func CutPrefixAndSuffix(s, prefix, suffix string) string {
	return strings.TrimPrefix(strings.TrimSuffix(s, suffix), prefix)
}
