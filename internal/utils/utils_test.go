package utils

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeServiceAccountJSON parses as Google service-account credentials. The
// key is never used to fetch a token, so a placeholder PEM body is enough.
const fakeServiceAccountJSON = `{
  "type": "service_account",
  "project_id": "fake-project",
  "private_key_id": "abc123",
  "private_key": "-----BEGIN PRIVATE KEY-----\nZmFrZQ==\n-----END PRIVATE KEY-----\n",
  "client_email": "fake@fake-project.iam.gserviceaccount.com",
  "client_id": "123",
  "token_uri": "https://oauth2.googleapis.com/token"
}`

func TestGetGoogleCredentials_CredentialsJSON(t *testing.T) {
	creds, err := GetGoogleCredentials(t.Context(), "", fakeServiceAccountJSON, "")
	if err != nil {
		t.Fatalf("GetGoogleCredentials with service-account JSON: %v", err)
	}
	if creds.ProjectID != "fake-project" {
		t.Errorf("ProjectID = %q, want %q", creds.ProjectID, "fake-project")
	}
}

func TestGetGoogleCredentials_InvalidJSON(t *testing.T) {
	if _, err := GetGoogleCredentials(t.Context(), "", "not-json", ""); err == nil {
		t.Fatal("GetGoogleCredentials with malformed JSON succeeded, want error")
	}
}

func TestGetGoogleCredentials_AccessTokenRequiresProject(t *testing.T) {
	if _, err := GetGoogleCredentials(t.Context(), "", "", "some-token"); err == nil {
		t.Fatal("GetGoogleCredentials with access token but no project succeeded, want error")
	}
}

func TestGetGoogleCredentials_AccessToken(t *testing.T) {
	creds, err := GetGoogleCredentials(t.Context(), "my-project", "", "some-token")
	if err != nil {
		t.Fatalf("GetGoogleCredentials with access token: %v", err)
	}
	if creds.ProjectID != "my-project" {
		t.Errorf("ProjectID = %q, want %q", creds.ProjectID, "my-project")
	}

	token, err := creds.TokenSource.Token()
	if err != nil {
		t.Fatalf("static token source: %v", err)
	}
	if token.AccessToken != "some-token" {
		t.Errorf("AccessToken = %q, want %q", token.AccessToken, "some-token")
	}
}

func TestGetGoogleCredentials_ApplicationDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "adc.json")
	if err := os.WriteFile(path, []byte(fakeServiceAccountJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)

	creds, err := GetGoogleCredentials(t.Context(), "", "", "")
	if err != nil {
		t.Fatalf("GetGoogleCredentials via ADC: %v", err)
	}
	if creds.ProjectID != "fake-project" {
		t.Errorf("ProjectID = %q, want %q", creds.ProjectID, "fake-project")
	}
}
