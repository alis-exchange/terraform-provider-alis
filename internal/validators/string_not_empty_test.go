package validators

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestStringNotEmpty(t *testing.T) {
	cases := map[string]struct {
		value     types.String
		wantError bool
	}{
		"null skipped":    {types.StringNull(), false},
		"unknown skipped": {types.StringUnknown(), false},
		"empty rejected":  {types.StringValue(""), true},
		"value accepted":  {types.StringValue("x"), false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			StringNotEmpty().ValidateString(t.Context(), validator.StringRequest{ConfigValue: tc.value}, resp)

			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Errorf("HasError() = %t, want %t (diagnostics: %v)", got, tc.wantError, resp.Diagnostics)
			}
		})
	}
}
