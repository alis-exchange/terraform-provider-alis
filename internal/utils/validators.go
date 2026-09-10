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
	projectIDRegex  = `^[a-z](?:[-a-z0-9]{4,28}[a-z0-9])?$`
	instanceIDRegex = `^[a-z0-9-]{6,33}$`
)

// Spanner regex.
var (
	spannerGoogleSQLDatabaseIDRegex   = `^[a-z][a-z0-9_\-]*[a-z0-9]{2,30}$`
	spannerPostgresSQLDatabaseIDRegex = `^[a-zA-Z][a-zA-Z0-9_]{2,30}$`
	SpannerGoogleSQLDatabaseNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s$`,
		CutPrefixAndSuffix(projectIDRegex, "^", "$"),
		CutPrefixAndSuffix(instanceIDRegex, "^", "$"),
		CutPrefixAndSuffix(spannerGoogleSQLDatabaseIDRegex, "^", "$"),
	)
	SpannerPostgresSQLDatabaseNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s$`,
		CutPrefixAndSuffix(projectIDRegex, "^", "$"),
		CutPrefixAndSuffix(instanceIDRegex, "^", "$"),
		CutPrefixAndSuffix(spannerPostgresSQLDatabaseIDRegex, "^", "$"),
	)
	// Role IDs name both database roles and the grantees of table
	// privileges, and reach DDL by concatenation — validate every one of them
	// against these before rendering GRANT/REVOKE/CREATE ROLE.
	SpannerGoogleSQLRoleIDRegex           = `^[a-zA-Z0-9_]{1,64}$`
	SpannerPostgresSQLRoleIDRegex         = `^[a-zA-Z0-9_]{1,64}$`
	SpannerGoogleSQLDatabaseRoleNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/databaseRoles\/%s$`,
		CutPrefixAndSuffix(projectIDRegex, "^", "$"),
		CutPrefixAndSuffix(instanceIDRegex, "^", "$"),
		CutPrefixAndSuffix(spannerGoogleSQLDatabaseIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSQLRoleIDRegex, "^", "$"),
	)
	SpannerPostgresSQLDatabaseRoleNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/databaseRoles\/%s$`,
		CutPrefixAndSuffix(projectIDRegex, "^", "$"),
		CutPrefixAndSuffix(instanceIDRegex, "^", "$"),
		CutPrefixAndSuffix(spannerPostgresSQLDatabaseIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSQLRoleIDRegex, "^", "$"),
	)
	SpannerGoogleSQLTableIDRegex   = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerPostgresSQLTableIDRegex = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerGoogleSQLTableNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s$`,
		CutPrefixAndSuffix(projectIDRegex, "^", "$"),
		CutPrefixAndSuffix(instanceIDRegex, "^", "$"),
		CutPrefixAndSuffix(spannerGoogleSQLDatabaseIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSQLTableIDRegex, "^", "$"),
	)
	SpannerPostgresSQLTableNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s$`,
		CutPrefixAndSuffix(projectIDRegex, "^", "$"),
		CutPrefixAndSuffix(instanceIDRegex, "^", "$"),
		CutPrefixAndSuffix(spannerPostgresSQLDatabaseIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSQLTableIDRegex, "^", "$"),
	)
	SpannerGoogleSQLTableRoleNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s\/tableRoles\/%s$`,
		CutPrefixAndSuffix(projectIDRegex, "^", "$"),
		CutPrefixAndSuffix(instanceIDRegex, "^", "$"),
		CutPrefixAndSuffix(spannerGoogleSQLDatabaseIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSQLTableIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSQLRoleIDRegex, "^", "$"),
	)
	SpannerPostgresSQLTableRoleNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s\/tableRoles\/%s$`,
		CutPrefixAndSuffix(projectIDRegex, "^", "$"),
		CutPrefixAndSuffix(instanceIDRegex, "^", "$"),
		CutPrefixAndSuffix(spannerPostgresSQLDatabaseIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSQLTableIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSQLRoleIDRegex, "^", "$"),
	)
	SpannerGoogleSQLColumnIDRegex       = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerPostgresSQLColumnIDRegex     = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerGoogleSQLIndexIDRegex        = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerPostgresSQLIndexIDRegex      = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerGoogleSQLTableIndexNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s\/indexes\/%s$`,
		CutPrefixAndSuffix(projectIDRegex, "^", "$"),
		CutPrefixAndSuffix(instanceIDRegex, "^", "$"),
		CutPrefixAndSuffix(spannerGoogleSQLDatabaseIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSQLTableIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSQLIndexIDRegex, "^", "$"),
	)
	SpannerPostgresSQLTableIndexNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s\/indexes\/%s$`,
		CutPrefixAndSuffix(projectIDRegex, "^", "$"),
		CutPrefixAndSuffix(instanceIDRegex, "^", "$"),
		CutPrefixAndSuffix(spannerPostgresSQLDatabaseIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSQLTableIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSQLIndexIDRegex, "^", "$"),
	)

	SpannerGoogleSQLConstraintIDRegex   = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerPostgresSQLConstraintIDRegex = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`

	SpannerGoogleSQLSequenceIDRegex   = `^[a-zA-Z0-9_]{1,64}$`
	SpannerPostgresSQLSequenceIDRegex = `^[a-zA-Z0-9_]{1,64}$`

	SpannerGoogleSQLSequenceNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/sequences\/%s$`,
		CutPrefixAndSuffix(projectIDRegex, "^", "$"),
		CutPrefixAndSuffix(instanceIDRegex, "^", "$"),
		CutPrefixAndSuffix(spannerGoogleSQLDatabaseIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerGoogleSQLSequenceIDRegex, "^", "$"),
	)
	SpannerPostgresSQLSequenceNameRegex = fmt.Sprintf(
		`^projects\/%s\/instances\/%s\/databases\/%s\/sequences\/%s$`,
		CutPrefixAndSuffix(projectIDRegex, "^", "$"),
		CutPrefixAndSuffix(instanceIDRegex, "^", "$"),
		CutPrefixAndSuffix(spannerPostgresSQLDatabaseIDRegex, "^", "$"),
		CutPrefixAndSuffix(SpannerPostgresSQLSequenceIDRegex, "^", "$"),
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
func ValidateDialectArgument(field, value, googleSQLRegex, postgresSQLRegex string) error {
	if ValidateArgument(value, googleSQLRegex) || ValidateArgument(value, postgresSQLRegex) {
		return nil
	}

	return status.Errorf(
		codes.InvalidArgument,
		"Invalid argument %s (%s), must match `%s` for GoogleSQL dialect or `%s` for PostgreSQL dialect",
		field,
		value,
		googleSQLRegex,
		postgresSQLRegex,
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
