package validators

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// String not empty Validator
var _ validator.String = stringNotEmptyValidator{}

// Non Empty String Validator
type stringNotEmptyValidator struct {
}

// Description describes the validation in plain text formatting.
func (v stringNotEmptyValidator) Description(_ context.Context) string {
	return "value expected to be a string that isn't an empty string"
}

// MarkdownDescription describes the validation in Markdown formatting.
func (v stringNotEmptyValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// ValidateString performs the validation.
func (v stringNotEmptyValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()

	if value == "" {
		response.Diagnostics.AddError("expected a non-empty string", fmt.Sprintf("%s was set to `%s`", request.Path, value))
	}
}

func StringNotEmpty() validator.String {
	return stringNotEmptyValidator{}
}
