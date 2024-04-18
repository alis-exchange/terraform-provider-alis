package validators

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = durationStringMinSeconds{}

// durationStringMaxSeconds validates that duration string is at least a certain number of seconds.
type durationStringMinSeconds struct {
	minDuration int
}

// Description describes the validation in plain text formatting.
func (validator durationStringMinSeconds) Description(_ context.Context) string {
	return fmt.Sprintf("duration must be at least %ds", validator.minDuration)
}

// MarkdownDescription describes the validation in Markdown formatting.
func (validator durationStringMinSeconds) MarkdownDescription(ctx context.Context) string {
	return validator.Description(ctx)
}

// ValidateString performs the validation.
func (v durationStringMinSeconds) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
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

	if duration < v.minDuration {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			request.Path,
			v.Description(ctx),
			value,
		))

		return
	}
}

// DurationStringMinSeconds returns a validator which ensures that any configured
// attribute value is a duration string greater than or equal to the given minimum.
func DurationStringMinSeconds(minDuration int) validator.String {
	if minDuration < 0 {
		return nil
	}

	return durationStringMinSeconds{
		minDuration: minDuration,
	}
}
