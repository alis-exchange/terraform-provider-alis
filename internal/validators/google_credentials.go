package validators

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	googleoauth "golang.org/x/oauth2/google"
)

// Credentials Validator
var _ validator.String = googleCredentialsValidator{}

// googleCredentialsValidator validates that a string Attribute's is valid JSON credentials.
type googleCredentialsValidator struct {
}

// Description describes the validation in plain text formatting.
func (v googleCredentialsValidator) Description(_ context.Context) string {
	return "value must be a path to valid JSON credentials or valid, raw, JSON credentials"
}

// MarkdownDescription describes the validation in Markdown formatting.
func (v googleCredentialsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// ValidateString performs the validation.
func (v googleCredentialsValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()

	// if this is a path and we can stat it, assume it's ok
	if _, err := os.Stat(value); err == nil {
		return
	}
	if _, err := googleoauth.CredentialsFromJSON(context.Background(), []byte(value)); err != nil {
		response.Diagnostics.AddError("JSON credentials are not valid", err.Error())
	}
}

func GoogleCredentialsValidator() validator.String {
	return googleCredentialsValidator{}
}
