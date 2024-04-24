package utils

import (
	"regexp"
)

var (
	ProjectIdRegex              = `^[a-z](?:[-a-z0-9]{4,28}[a-z0-9])?$`
	InstanceIdRegex             = `^[a-z0-9-]{6,33}$`
	InstanceNameRegex           = `^projects\/[a-z](?:[-a-z0-9]{4,28}[a-z0-9])?\/instances\/[a-z0-9-]{6,33}$`
	BigtableTableIdRegex        = `^[a-zA-Z0-9_.-]{1,50}$`
	BigtableTableNameRegex      = `^projects\/[a-z](?:[-a-z0-9]{4,28}[a-z0-9])?\/instances\/[a-z0-9-]{6,33}\/tables\/[a-zA-Z0-9_.-]{1,50}$`
	BigtableColumnFamilyIdRegex = `^[-_.a-zA-Z0-9]{1,50}$`
	BigtableClusterIdRegex      = `^[a-z0-9-]{6,30}$`
	BigtableClusterNameRegex    = `^projects\/[a-z](?:[-a-z0-9]{4,28}[a-z0-9])?\/instances\/[a-z0-9-]{6,33}\/clusters\/[a-z0-9-]{6,30}$`
	BigtableBackupIdRegex       = `^[a-zA-Z0-9_.-]{1,50}$`
	BigtableBackupNameRegex     = `^projects\/[a-z](?:[-a-z0-9]{4,28}[a-z0-9])?\/instances\/[a-z0-9-]{6,33}\/clusters\/[a-z0-9-]{6,30}\/backups\/[a-zA-Z0-9_.-]{1,50}$`

	SpannerDatabaseIdRegex   = `^[a-z][a-z0-9_\-]*[a-z0-9]{2,60}$`
	SpannerDatabaseNameRegex = `^projects\/[a-z](?:[-a-z0-9]{4,28}[a-z0-9])?\/instances\/[a-z0-9-]{6,33}\/databases\/[a-z][a-z0-9_\-]*[a-z0-9]{2,60}$`
	SpannerTableIdRegex      = `^[a-zA-Z0-9_-]{1,50}$`
	SpannerTableNameRegex    = `^projects\/[a-z](?:[-a-z0-9]{4,28}[a-z0-9])?\/instances\/[a-z0-9-]{6,33}\/databases\/[a-z][a-z0-9_\-]*[a-z0-9]{2,60}\/tables\/[a-zA-Z0-9_-]{1,50}$`
	SpannerBackupIdRegex     = `^[a-z][a-z0-9_\-]*[a-z0-9]{2,60}$`
	SpannerBackupNameRegex   = `^projects\/[a-z](?:[-a-z0-9]{4,28}[a-z0-9])?\/instances\/[a-z0-9-]{6,33}\/backups\/[a-z][a-z0-9_\-]*[a-z0-9]{2,60}$`
	SpannerColumnIdRegex     = `^[a-zA-Z0-9_-]{1,50}$`
)

// ValidateArgument validates an argument against the provided regex and returns either true or false
func ValidateArgument(value string, regex string) bool {
	// Validate the value field using regex
	validateName := regexp.MustCompile(regex)
	validatedName := validateName.MatchString(value)

	return validatedName
}
