package schema

import (
	"fmt"
	"strings"
)

// CreateRoleDdl renders the CREATE ROLE statement.
func CreateRoleDdl(roleId string) string {
	return fmt.Sprintf("CREATE ROLE %s", roleId)
}

// DropRoleDdl renders the DROP ROLE statement.
func DropRoleDdl(roleId string) string {
	return fmt.Sprintf("DROP ROLE %s", roleId)
}

// GrantTablePrivilegesDdl renders the GRANT statement for table permissions
// (e.g. SELECT, INSERT, UPDATE, DELETE), joined in the given order.
func GrantTablePrivilegesDdl(table string, role string, permissions []string) string {
	return fmt.Sprintf("GRANT %s ON TABLE %s TO ROLE %s", strings.Join(permissions, ", "), table, role)
}

// RevokeTablePrivilegesDdl renders the REVOKE statement for table permissions.
func RevokeTablePrivilegesDdl(table string, role string, permissions []string) string {
	return fmt.Sprintf("REVOKE %s ON TABLE %s FROM ROLE %s", strings.Join(permissions, ", "), table, role)
}
