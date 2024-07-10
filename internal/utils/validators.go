package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// Regex for project and instance
var (
	ProjectIdRegex  = `^[a-z](?:[-a-z0-9]{4,28}[a-z0-9])?$`
	InstanceIdRegex = `^[a-z0-9-]{6,33}$`
)

// Bigtable regex
var (
	InstanceNameRegex           = fmt.Sprintf(`^projects\/%s\/instances\/%s$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(InstanceIdRegex, "^", "$"))
	BigtableTableIdRegex        = `^[a-zA-Z0-9_.-]{1,50}$`
	BigtableTableNameRegex      = fmt.Sprintf(`^projects\/%s\/instances\/%s\/tables\/%s$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(InstanceIdRegex, "^", "$"), CutPrefixAndSuffix(BigtableTableIdRegex, "^", "$"))
	BigtableColumnFamilyIdRegex = `^[-_.a-zA-Z0-9]{1,50}$`
	BigtableClusterIdRegex      = `^[a-z0-9-]{6,30}$`
	BigtableClusterNameRegex    = fmt.Sprintf(`^projects\/%s\/instances\/%s\/clusters\/%s$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(InstanceIdRegex, "^", "$"), CutPrefixAndSuffix(BigtableClusterIdRegex, "^", "$"))
	BigtableBackupIdRegex       = `^[a-zA-Z0-9_.-]{1,50}$`
	BigtableBackupNameRegex     = fmt.Sprintf(`^projects\/%s\/instances\/%s\/clusters\/%s\/backups\/%s$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(InstanceIdRegex, "^", "$"), CutPrefixAndSuffix(BigtableClusterIdRegex, "^", "$"), CutPrefixAndSuffix(BigtableBackupIdRegex, "^", "$"))
)

// Spanner regex
var (
	SpannerGoogleSqlDatabaseIdRegex         = `^[a-z][a-z0-9_\-]*[a-z0-9]{2,30}$`
	SpannerPostgresSqlDatabaseIdRegex       = `^[a-zA-Z][a-zA-Z0-9_]{2,30}$`
	SpannerGoogleSqlDatabaseNameRegex       = fmt.Sprintf(`^projects\/%s\/instances\/%s\/databases\/%s$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(InstanceIdRegex, "^", "$"), CutPrefixAndSuffix(SpannerGoogleSqlDatabaseIdRegex, "^", "$"))
	SpannerPostgresSqlDatabaseNameRegex     = fmt.Sprintf(`^projects\/%s\/instances\/%s\/databases\/%s$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(InstanceIdRegex, "^", "$"), CutPrefixAndSuffix(SpannerPostgresSqlDatabaseIdRegex, "^", "$"))
	SpannerGoogleSqlDatabaseRoleNameRegex   = fmt.Sprintf(`^projects\/%s\/instances\/%s\/databases\/%s\/databaseRoles\/[a-zA-Z0-9_]{1,64}$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(InstanceIdRegex, "^", "$"), CutPrefixAndSuffix(SpannerGoogleSqlDatabaseIdRegex, "^", "$"))
	SpannerPostgresSqlDatabaseRoleNameRegex = fmt.Sprintf(`^projects\/%s\/instances\/%s\/databases\/%s\/databaseRoles\/[a-zA-Z0-9_]{1,64}$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(InstanceIdRegex, "^", "$"), CutPrefixAndSuffix(SpannerGoogleSqlDatabaseIdRegex, "^", "$"))
	SpannerGoogleSqlTableIdRegex            = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerPostgresSqlTableIdRegex          = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerGoogleSqlTableNameRegex          = fmt.Sprintf(`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(InstanceIdRegex, "^", "$"), CutPrefixAndSuffix(SpannerGoogleSqlDatabaseIdRegex, "^", "$"), CutPrefixAndSuffix(SpannerGoogleSqlTableIdRegex, "^", "$"))
	SpannerPostgresSqlTableNameRegex        = fmt.Sprintf(`^projects\/%s\/instances\/%s\/databases\/%s\/tables\/%s$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(InstanceIdRegex, "^", "$"), CutPrefixAndSuffix(SpannerPostgresSqlDatabaseIdRegex, "^", "$"), CutPrefixAndSuffix(SpannerPostgresSqlTableIdRegex, "^", "$"))
	SpannerGoogleSqlBackupIdRegex           = `^[a-z][a-z0-9_\-]*[a-z0-9]{2,30}$`
	SpannerPostgresSqlBackupIdRegex         = `^[a-zA-Z][a-zA-Z0-9_]{2,30}$`
	SpannerGoogleSqlBackupNameRegex         = fmt.Sprintf(`^projects\/%s\/instances\/%s\/backups\/%s$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(InstanceIdRegex, "^", "$"), CutPrefixAndSuffix(SpannerGoogleSqlBackupIdRegex, "^", "$"))
	SpannerPostgresSqlBackupNameRegex       = fmt.Sprintf(`^projects\/%s\/instances\/%s\/backups\/%s$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(InstanceIdRegex, "^", "$"), CutPrefixAndSuffix(SpannerPostgresSqlBackupIdRegex, "^", "$"))
	SpannerGoogleSqlColumnIdRegex           = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerPostgresSqlColumnIdRegex         = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerGoogleSqlIndexIdRegex            = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`
	SpannerPostgresSqlIndexIdRegex          = `^[a-zA-Z][a-zA-Z0-9_]{0,127}$`

	SpannerGoogleSqlConstraintIdRegex   = ``
	SpannerPostgresSqlConstraintIdRegex = ``

	DiscoveryEngineDatastoreNameRegex       = fmt.Sprintf(`^projects\/%s\/locations\/[a-zA-Z0-9-]*\/collections\/[a-zA-Z0-9-_]*\/dataStores\/[a-z0-9-_]*$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"))
	DiscoveryEngineDatastoreSchemaIdRegex   = `^[a-zA-Z0-9-_]*$`
	DiscoveryEngineDatastoreSchemaNameRegex = fmt.Sprintf(`^projects\/%s\/locations\/[a-zA-Z0-9-]*\/collections\/[a-zA-Z0-9-_]*\/dataStores\/[a-z0-9-_]*\/schemas\/%s$`, CutPrefixAndSuffix(ProjectIdRegex, "^", "$"), CutPrefixAndSuffix(DiscoveryEngineDatastoreSchemaIdRegex, "^", "$"))
)

// ValidateArgument validates an argument against the provided regex and returns either true or false
func ValidateArgument(value string, regex string) bool {
	// Validate the value field using regex
	validateName := regexp.MustCompile(regex)
	validatedName := validateName.MatchString(value)

	return validatedName
}

// CutPrefixAndSuffix cuts the prefix and suffix from a string
// If the prefix or suffix is not present, the string is returned as is
func CutPrefixAndSuffix(s string, prefix string, suffix string) string {
	return strings.TrimPrefix(strings.TrimSuffix(s, suffix), prefix)
}
