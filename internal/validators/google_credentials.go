package validators

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	googleoauth "golang.org/x/oauth2/google"
)

// Credentials Validator.
var _ validator.String = googleCredentialsValidator{}

// googleCredentialsValidator validates that a string Attribute's is valid JSON credentials.
type googleCredentialsValidator struct{}

// Description describes the validation in plain text formatting.
func (v googleCredentialsValidator) Description(_ context.Context) string {
	return "value must be a path to valid JSON credentials or valid, raw, JSON credentials"
}

// MarkdownDescription describes the validation in Markdown formatting.
func (v googleCredentialsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// ValidateString performs the validation.
func (v googleCredentialsValidator) ValidateString(
	ctx context.Context,
	request validator.StringRequest,
	response *validator.StringResponse,
) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()

	// If this is a path and we can stat it, assume it's ok; unreadable or
	// malformed file contents still fail later, at provider Configure.
	if _, err := os.Stat(value); err == nil {
		return
	}
	//nolint:staticcheck // deprecated parse API; validation-only use until the provider migrates to cloud.google.com/go/auth
	if _, err := googleoauth.CredentialsFromJSON(ctx, []byte(value)); err != nil {
		// Deliberately does not echo the value: credentials are secret.
		response.Diagnostics.AddAttributeError(
			request.Path,
			"Invalid Google Credentials",
			"Value is neither a path to an existing credentials file nor valid Google credentials JSON: "+err.Error(),
		)
	}
}

// GoogleCredentialsValidator returns a validator which ensures that a
// configured credentials string is either a path to an existing file (assumed
// to hold credentials) or raw JSON that parses as Google credentials. Null
// (unconfigured) and unknown (known after apply) values are skipped.
func GoogleCredentialsValidator() validator.String {
	return googleCredentialsValidator{}
}
