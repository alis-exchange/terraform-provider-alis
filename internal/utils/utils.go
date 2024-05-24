package utils

import (
	"context"
	"errors"
	"strings"

	"cloud.google.com/go/bigtable"
	discoveryengine "cloud.google.com/go/discoveryengine/apiv1beta"
	"cloud.google.com/go/spanner"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	oath2 "golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
)

// ItemInSlice checks if an item is in a slice
// Item must be comparable
func ItemInSlice[T comparable](list []T, item T) bool {
	// TODO: Is there a better way to do this than brute force?
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}

// SnakeCaseToPascalCase converts a snake_case string to PascalCase
func SnakeCaseToPascalCase(s string) string {
	// Split the string at each underscore, which separates words in snake_case.
	words := strings.Split(s, "_")

	// Capitalize the first letter of each word and join them.
	for i, word := range words {
		// Use strings.Title to capitalize the first letter of each word.
		// Note: strings.Title could capitalize other letters in some cases, so better approach is to use this:
		words[i] = strings.ToUpper(string(word[0])) + word[1:]
	}

	// Join the capitalized words without any separators.
	return strings.Join(words, "")
}

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
// Returns: {google.Credentials}
func GetGoogleCredentials(ctx context.Context, projectId string, credentialsStr string, accessToken string, scopes ...string) (*googleoauth.Credentials, error) {

	// Set default scopes if none are provided
	if len(scopes) == 0 {
		scopes = []string{
			bigtable.Scope,
			bigtable.AdminScope,
			bigtable.InstanceAdminScope,
			spanner.Scope,
			spanner.AdminScope,
		}
		scopes = append(scopes, discoveryengine.DefaultAuthScopes()...)
	}

	// If credentialsStr are provided, use them
	if credentialsStr != "" {
		creds, err := googleoauth.CredentialsFromJSON(ctx, []byte(credentialsStr), scopes...)
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
	creds, err := googleoauth.FindDefaultCredentials(ctx, scopes...)
	if err != nil {
		return nil, err
	}

	tflog.Debug(ctx, "Using Application Default Credentials")
	return creds, nil
}
