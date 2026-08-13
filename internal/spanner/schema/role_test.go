package schema

import "testing"

func TestRoleAndGrantDdl(t *testing.T) {
	t.Run("CreateRoleDdl", func(t *testing.T) {
		if got := CreateRoleDdl("inventory_admin"); got != "CREATE ROLE inventory_admin" {
			t.Errorf("CreateRoleDdl() = %q", got)
		}
	})

	t.Run("DropRoleDdl", func(t *testing.T) {
		if got := DropRoleDdl("inventory_admin"); got != "DROP ROLE inventory_admin" {
			t.Errorf("DropRoleDdl() = %q", got)
		}
	})

	t.Run("GrantTablePrivilegesDdl joins permissions in order", func(t *testing.T) {
		got := GrantTablePrivilegesDdl("inventory", "inventory_admin", []string{"SELECT", "INSERT", "UPDATE", "DELETE"})
		want := "GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE inventory TO ROLE inventory_admin"
		if got != want {
			t.Errorf("GrantTablePrivilegesDdl() = %q, want %q", got, want)
		}
	})

	t.Run("RevokeTablePrivilegesDdl", func(t *testing.T) {
		got := RevokeTablePrivilegesDdl("inventory", "inventory_admin", []string{"SELECT"})
		want := "REVOKE SELECT ON TABLE inventory FROM ROLE inventory_admin"
		if got != want {
			t.Errorf("RevokeTablePrivilegesDdl() = %q, want %q", got, want)
		}
	})
}
