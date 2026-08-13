package utils

import (
	"context"
	"errors"

	"cloud.google.com/go/spanner"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	oath2 "golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
)

// GetGoogleCredentials retrieves google.Credentials from the provided credentials, access token, or application default credentials.
// The source priority is as follows:
//  1. Credentials
//  2. Access Token
//  3. Application Default Credentials(ADC)
//
// If no credentials are provided, it will attempt to the access token.
// If both are missing, it will attempt to use the Application Default Credentials.
//
// Params:
//   - ctx: {context.Context} - The context to use for the operation(Required)
//   - projectId: {string} - The Google Cloud project ID(Required for accessToken)
//   - credentialsStr: {string} - The credentials JSON string
//   - accessToken: {string} - The access token
//   - scopes: {[]string} - The scopes to use for the credentials
//
// Returns: {google.Credentials}.
func GetGoogleCredentials(
	ctx context.Context,
	projectId, credentialsStr, accessToken string,
	scopes ...string,
) (*googleoauth.Credentials, error) {
	// Set default scopes if none are provided
	if len(scopes) == 0 {
		scopes = []string{
			spanner.Scope,
			spanner.AdminScope,
		}
	}

	// The returned token source outlives the calling RPC: it is cached in
	// clients for the provider's whole lifetime and fetches tokens lazily.
	// oauth2 binds the token-refresh HTTP client to the context given here,
	// so a request-scoped ctx would make every refresh after that request
	// fail with "context canceled".
	tokenCtx := context.Background()

	// If credentialsStr are provided, use them
	if credentialsStr != "" {
		//nolint:contextcheck,staticcheck // tokenCtx deliberately outlives the request; deprecated parse API kept until migration to cloud.google.com/go/auth
		creds, err := googleoauth.CredentialsFromJSON(tokenCtx, []byte(credentialsStr), scopes...)
		if err != nil {
			return nil, err
		}

		tflog.Debug(ctx, "Using provided credentials")
		return creds, nil
	}

	// If access token is provided, use it
	if accessToken != "" {
		// Ensure that projectId is provided
		if projectId == "" {
			return nil, errors.New("projectId is required for accessToken")
		}

		staticTokenSource := oath2.StaticTokenSource(&oath2.Token{
			AccessToken: accessToken,
		})

		tflog.Debug(ctx, "Using provided access token")
		return &googleoauth.Credentials{
			ProjectID:   projectId,
			TokenSource: staticTokenSource,
		}, nil
	}

	// If no credentialsStr or access token is provided, use Application Default Credentials
	//nolint:contextcheck // tokenCtx deliberately outlives the request; see comment above
	creds, err := googleoauth.FindDefaultCredentials(tokenCtx, scopes...)
	if err != nil {
		return nil, err
	}

	tflog.Debug(ctx, "Using Application Default Credentials")
	return creds, nil
}
