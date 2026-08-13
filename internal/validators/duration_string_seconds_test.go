package validators

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDurationStringMinSeconds(t *testing.T) {
	cases := map[string]struct {
		value     types.String
		wantError bool
	}{
		"null skipped":     {types.StringNull(), false},
		"unknown skipped":  {types.StringUnknown(), false},
		"above minimum":    {types.StringValue("600s"), false},
		"exactly minimum":  {types.StringValue("300s"), false},
		"below minimum":    {types.StringValue("60s"), true},
		"not a duration":   {types.StringValue("abc"), true},
		"missing integer":  {types.StringValue("s"), true},
		"fractional value": {types.StringValue("1.5s"), true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			DurationStringMinSeconds(300).ValidateString(t.Context(), validator.StringRequest{ConfigValue: tc.value}, resp)

			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Errorf("HasError() = %t, want %t (diagnostics: %v)", got, tc.wantError, resp.Diagnostics)
			}
		})
	}
}

func TestDurationStringMinSeconds_NegativeMinimum(t *testing.T) {
	if v := DurationStringMinSeconds(-1); v != nil {
		t.Errorf("DurationStringMinSeconds(-1) = %v, want nil", v)
	}
}

func TestDurationStringMaxSeconds(t *testing.T) {
	cases := map[string]struct {
		value     types.String
		wantError bool
	}{
		"null skipped":    {types.StringNull(), false},
		"unknown skipped": {types.StringUnknown(), false},
		"below maximum":   {types.StringValue("600s"), false},
		"exactly maximum": {types.StringValue("3600s"), false},
		"above maximum":   {types.StringValue("7200s"), true},
		"not a duration":  {types.StringValue("abc"), true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			DurationStringMaxSeconds(3600).ValidateString(t.Context(), validator.StringRequest{ConfigValue: tc.value}, resp)

			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Errorf("HasError() = %t, want %t (diagnostics: %v)", got, tc.wantError, resp.Diagnostics)
			}
		})
	}
}

func TestDurationStringMaxSeconds_NegativeMaximum(t *testing.T) {
	if v := DurationStringMaxSeconds(-1); v != nil {
		t.Errorf("DurationStringMaxSeconds(-1) = %v, want nil", v)
	}
}
