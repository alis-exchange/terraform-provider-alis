package validators

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const testServiceAccountJSON = `{
  "type": "service_account",
  "project_id": "fake-project",
  "private_key_id": "abc123",
  "private_key": "-----BEGIN PRIVATE KEY-----\nZmFrZQ==\n-----END PRIVATE KEY-----\n",
  "client_email": "fake@fake-project.iam.gserviceaccount.com",
  "client_id": "123",
  "token_uri": "https://oauth2.googleapis.com/token"
}`

func TestGoogleCredentialsValidator(t *testing.T) {
	existingFile := filepath.Join(t.TempDir(), "creds.json")
	if err := os.WriteFile(existingFile, []byte("not even json"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct {
		value     types.String
		wantError bool
	}{
		"null skipped":    {types.StringNull(), false},
		"unknown skipped": {types.StringUnknown(), false},
		// An existing file passes on the stat alone; contents are checked
		// later, at provider Configure.
		"existing file path": {types.StringValue(existingFile), false},
		"valid JSON":         {types.StringValue(testServiceAccountJSON), false},
		"invalid JSON":       {types.StringValue("not-json-and-not-a-file"), true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			GoogleCredentialsValidator().ValidateString(t.Context(), validator.StringRequest{ConfigValue: tc.value}, resp)

			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Errorf("HasError() = %t, want %t (diagnostics: %v)", got, tc.wantError, resp.Diagnostics)
			}
		})
	}
}
