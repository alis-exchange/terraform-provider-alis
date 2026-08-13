package schema

import (
	"fmt"
	"strings"
)

// CreateProtoBundleDdl renders the CREATE PROTO BUNDLE statement for the
// given already-quoted proto package names.
func CreateProtoBundleDdl(quotedNames []string) string {
	return fmt.Sprintf("CREATE PROTO BUNDLE (%s)", strings.Join(quotedNames, ", "))
}

// AlterProtoBundleInsertDdl renders the ALTER PROTO BUNDLE INSERT statement.
func AlterProtoBundleInsertDdl(quotedNames []string) string {
	return fmt.Sprintf("ALTER PROTO BUNDLE INSERT (%s)", strings.Join(quotedNames, ", "))
}

// AlterProtoBundleUpdateDdl renders the ALTER PROTO BUNDLE UPDATE statement.
func AlterProtoBundleUpdateDdl(quotedNames []string) string {
	return fmt.Sprintf("ALTER PROTO BUNDLE UPDATE (%s)", strings.Join(quotedNames, ", "))
}
