package provider

import (
	"testing"

	"terraform-provider-alis/internal/spanner/conn"
	"terraform-provider-alis/internal/spanner/conn/connfake"

	googleoauth "golang.org/x/oauth2/google"
)

// resetConnections empties the process-wide cache so one test's entries cannot
// satisfy another's lookups.
func resetConnections(t *testing.T) {
	t.Helper()

	connectionsMu.Lock()
	defer connectionsMu.Unlock()
	clear(connections)
}

// The cache key has to cover every input the Connection is built from. The two
// that are easy to forget are the ones that do not appear in the provider
// configuration at all: the identity ADC resolved to, and the emulator host
// conn captures at construction.
func TestConnectionKey_SeparatesEveryInput(t *testing.T) {
	adc := &googleoauth.Credentials{ProjectID: "my-project", JSON: []byte(`{"client_email":"first@example.com"}`)}

	// The key includes the ambient emulator host; pin it so the base key is
	// stable even when the test process runs with SPANNER_EMULATOR_HOST set
	// (as the emulator-backed acceptance suite does).
	t.Setenv("SPANNER_EMULATOR_HOST", "")
	base := connectionKey("my-project", "", "", adc)

	tests := []struct {
		name         string
		project      string
		credentials  string
		accessToken  string
		resolved     *googleoauth.Credentials
		emulatorHost string
		wantSameKey  bool
	}{
		{
			name:        "identical inputs",
			project:     "my-project",
			resolved:    adc,
			wantSameKey: true,
		},
		{
			name:     "different project",
			project:  "other-project",
			resolved: adc,
		},
		{
			name:        "different credentials",
			project:     "my-project",
			credentials: `{"type":"service_account"}`,
			resolved:    adc,
		},
		{
			name:        "different access token",
			project:     "my-project",
			accessToken: "token",
			resolved:    adc,
		},
		{
			// Same configuration, different principal: ADC can name another
			// service account without a line of Terraform changing.
			name:     "same config, different resolved identity",
			project:  "my-project",
			resolved: &googleoauth.Credentials{ProjectID: "my-project", JSON: []byte(`{"client_email":"second@example.com"}`)},
		},
		{
			// conn reads this at construction, so it decides which backend the
			// Connection talks to.
			name:         "different emulator host",
			project:      "my-project",
			resolved:     adc,
			emulatorHost: "localhost:9010",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SPANNER_EMULATOR_HOST", tc.emulatorHost)

			got := connectionKey(tc.project, tc.credentials, tc.accessToken, tc.resolved)
			if same := got == base; same != tc.wantSameKey {
				t.Errorf("key equals base = %v, want %v", same, tc.wantSameKey)
			}
		})
	}
}

// Nil credentials cannot reach here through Configure, but a panic in a
// provider surfaces to practitioners as "the plugin crashed".
func TestConnectionKey_ToleratesNilCredentials(t *testing.T) {
	if connectionKey("my-project", "", "", nil) == ([32]byte{}) {
		t.Error("connectionKey returned the zero key")
	}
}

func TestSharedConnection_ReusesOneConnectionPerKey(t *testing.T) {
	resetConnections(t)
	t.Cleanup(func() { resetConnections(t) })

	builds := 0
	build := func() conn.Connection {
		builds++
		return connfake.New()
	}

	first := sharedConnection([32]byte{1}, build)
	second := sharedConnection([32]byte{1}, build)
	other := sharedConnection([32]byte{2}, build)

	if first != second {
		t.Error("same key returned different Connections")
	}
	if first == other {
		t.Error("different keys returned the same Connection")
	}
	if builds != 2 {
		t.Errorf("built %d Connections, want 2 (one per distinct key)", builds)
	}
}
