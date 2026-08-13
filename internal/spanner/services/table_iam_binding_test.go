package services

import (
	"context"
	"strings"
	"testing"

	"terraform-provider-alis/internal/spanner/conn/connfake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	testDatabase = "projects/test-project/instances/test-instance/databases/test-db"
	testTable    = testDatabase + "/tables/tftest_table"
	testRole     = "tftest_role"
)

// privilegeRows renders what INFORMATION_SCHEMA.TABLE_PRIVILEGES returns for a
// role already holding the given permissions.
func privilegeRows(permissions ...string) []*TablePermissionsRow {
	rows := make([]*TablePermissionsRow, 0, len(permissions))
	for _, permission := range permissions {
		rows = append(rows, &TablePermissionsRow{
			TABLE_NAME:     "tftest_table",
			PRIVILEGE_TYPE: permission,
			GRANTEE:        testRole,
		})
	}

	return rows
}

// A binding is authoritative for its role, so the DDL it issues is whatever
// closes the gap between what Spanner holds and what the caller asked for.
// Granting without revoking would leave the role over-privileged while state
// claimed otherwise — a silent, permanent drift.
func TestSetTableIamBinding_MatchesRequestedPermissions(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		want     []TablePolicyBindingPermission
		wantDdl  []string
	}{
		{
			name:     "grants everything when the role holds nothing",
			existing: nil,
			want: []TablePolicyBindingPermission{
				TablePolicyBindingPermission_SELECT,
				TablePolicyBindingPermission_INSERT,
			},
			wantDdl: []string{"GRANT SELECT, INSERT ON TABLE tftest_table TO ROLE tftest_role"},
		},
		{
			name:     "revokes the permission dropped from the request",
			existing: []string{"SELECT", "INSERT", "UPDATE", "DELETE"},
			want: []TablePolicyBindingPermission{
				TablePolicyBindingPermission_SELECT,
				TablePolicyBindingPermission_INSERT,
				TablePolicyBindingPermission_UPDATE,
			},
			wantDdl: []string{"REVOKE DELETE ON TABLE tftest_table FROM ROLE tftest_role"},
		},
		{
			name:     "grants and revokes in one batch",
			existing: []string{"SELECT", "DELETE"},
			want: []TablePolicyBindingPermission{
				TablePolicyBindingPermission_SELECT,
				TablePolicyBindingPermission_UPDATE,
			},
			wantDdl: []string{
				"REVOKE DELETE ON TABLE tftest_table FROM ROLE tftest_role",
				"GRANT UPDATE ON TABLE tftest_table TO ROLE tftest_role",
			},
		},
		{
			name:     "issues nothing when the grants already match",
			existing: []string{"SELECT"},
			want:     []TablePolicyBindingPermission{TablePolicyBindingPermission_SELECT},
			wantDdl:  nil,
		},
		{
			name:     "orders operands by permission, not by request",
			existing: nil,
			want: []TablePolicyBindingPermission{
				TablePolicyBindingPermission_DELETE,
				TablePolicyBindingPermission_SELECT,
			},
			wantDdl: []string{"GRANT SELECT, DELETE ON TABLE tftest_table TO ROLE tftest_role"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := connfake.New()
			fake.OnQuery("TABLE_PRIVILEGES", privilegeRows(tc.existing...))

			_, err := NewSpannerService(fake).SetTableIamBinding(context.Background(), testTable, &TablePolicyBinding{
				Role:        testRole,
				Permissions: tc.want,
			})
			require.NoError(t, err)

			var ddl []string
			for _, statement := range fake.Statements() {
				if strings.HasPrefix(statement, "GRANT") || strings.HasPrefix(statement, "REVOKE") {
					ddl = append(ddl, statement)
				}
			}
			assert.Equal(t, tc.wantDdl, ddl)
		})
	}
}

// The role reaches GRANT/REVOKE by string concatenation, so anything but a
// bare identifier would let a config grant privileges it never declared.
func TestSetTableIamBinding_RejectsRoleThatIsNotAnIdentifier(t *testing.T) {
	for _, role := range []string{"admin1, admin2", "admin; DROP TABLE t", "", "admin-1"} {
		t.Run(role, func(t *testing.T) {
			fake := connfake.New()
			fake.OnQuery("TABLE_PRIVILEGES", privilegeRows())

			_, err := NewSpannerService(fake).SetTableIamBinding(context.Background(), testTable, &TablePolicyBinding{
				Role:        role,
				Permissions: []TablePolicyBindingPermission{TablePolicyBindingPermission_SELECT},
			})

			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
			assert.Empty(t, fake.Statements(), "no DDL may be issued for an invalid role")
		})
	}
}

// Destroying a binding whose grants are already gone must converge rather than
// leaving the resource wedged in state.
func TestDeleteTableIamBinding_ToleratesMissingGrants(t *testing.T) {
	fake := connfake.New()
	fake.OnQuery("TABLE_PRIVILEGES", privilegeRows())

	require.NoError(t, NewSpannerService(fake).DeleteTableIamBinding(context.Background(), testTable, testRole))
	assert.Empty(t, fake.Statements(), "an empty REVOKE is not valid DDL")
}

func TestDeleteTableIamBinding_RevokesEveryGrantedPermission(t *testing.T) {
	fake := connfake.New()
	fake.OnQuery("TABLE_PRIVILEGES", privilegeRows("DELETE", "SELECT"))

	require.NoError(t, NewSpannerService(fake).DeleteTableIamBinding(context.Background(), testTable, testRole))
	assert.Equal(t, []string{"REVOKE SELECT, DELETE ON TABLE tftest_table FROM ROLE tftest_role"}, fake.Statements())
}
