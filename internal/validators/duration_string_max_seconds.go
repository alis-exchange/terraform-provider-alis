package validators

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = durationStringMaxSeconds{}

// durationStringMaxSeconds validates that a duration string is at most a
// certain number of seconds. The whole number of seconds preceding the first
// "s" is compared (e.g. "300s"); values that do not start with an integer are
// rejected.
type durationStringMaxSeconds struct {
	maxDuration int
}

// Description describes the validation in plain text formatting.
func (v durationStringMaxSeconds) Description(_ context.Context) string {
	return fmt.Sprintf("duration must be at most %ds", v.maxDuration)
}

// MarkdownDescription describes the validation in Markdown formatting.
func (v durationStringMaxSeconds) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// ValidateString performs the validation.
func (v durationStringMaxSeconds) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	// Get the value
	value := request.ConfigValue.ValueString()
	// Split the value
	valueParts := strings.Split(value, "s")

	// Parse the duration to int
	duration, err := strconv.Atoi(valueParts[0])
	if err != nil {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			request.Path,
			v.Description(ctx),
			value,
		))

		return
	}

	if duration > v.maxDuration {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			request.Path,
			v.Description(ctx),
			value,
		))

		return
	}
}

// DurationStringMaxSeconds returns a validator which ensures that any configured
// attribute value is a duration string less than or equal to the given maximum.
func DurationStringMaxSeconds(maxDuration int) validator.String {
	if maxDuration < 0 {
		return nil
	}

	return durationStringMaxSeconds{
		maxDuration: maxDuration,
	}
}
