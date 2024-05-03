package validators

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = regexMatchesValidator{}

// regexMatchesValidator validates that a string Attribute's value matches the specified regular expression.
type regexMatchesValidator struct {
	regexps []*regexp.Regexp
	message string
}

// Description describes the validation in plain text formatting.
func (validator regexMatchesValidator) Description(_ context.Context) string {
	if validator.message != "" {
		return validator.message
	}
	return fmt.Sprintf("value must match one of the regular expressions '%s'", validator.regexps)
}

// MarkdownDescription describes the validation in Markdown formatting.
func (validator regexMatchesValidator) MarkdownDescription(ctx context.Context) string {
	return validator.Description(ctx)
}

// Validate performs the validation.
func (v regexMatchesValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()

	atLeastOneValid := false
	for _, r := range v.regexps {
		atLeastOneValid = r.MatchString(value)
	}

	if !atLeastOneValid {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueMatchDiagnostic(
			request.Path,
			v.Description(ctx),
			value,
		))
	}
}

// RegexMatches returns an AttributeValidator which ensures that any configured
// attribute value:
//
//   - Is a string.
//   - Matches at least one of the given regular expression https://github.com/google/re2/wiki/Syntax.
//
// Null (unconfigured) and unknown (known after apply) values are skipped.
// Optionally an error message can be provided to return something friendlier
// than "value must match regular expression 'regexp'".
func RegexMatches(regexps []*regexp.Regexp, message string) validator.String {
	return regexMatchesValidator{
		regexps: regexps,
		message: message,
	}
}
