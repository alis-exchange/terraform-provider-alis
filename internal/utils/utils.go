package utils

import (
	"context"
	"strings"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/spanner"
	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func GetCredentials(ctx context.Context, scopes ...string) (*googleoauth.Credentials, error) {
	if len(scopes) == 0 {
		scopes = []string{bigtable.Scope, spanner.Scope}
	}

	creds, err := googleoauth.FindDefaultCredentials(ctx, scopes...)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, err
		}
	}

	return creds, nil
}
