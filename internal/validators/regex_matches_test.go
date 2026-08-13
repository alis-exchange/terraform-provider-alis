package validators

import (
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRegexMatches(t *testing.T) {
	// Two deliberately disjoint patterns so a value can match exactly one.
	regexps := []*regexp.Regexp{
		regexp.MustCompile(`^first_[a-z]+$`),
		regexp.MustCompile(`^second_[a-z]+$`),
	}

	cases := []struct {
		name    string
		value   types.String
		wantErr bool
	}{
		// A value matching only the FIRST pattern must pass: any single
		// match is sufficient.
		{name: "matches first only", value: types.StringValue("first_abc"), wantErr: false},
		{name: "matches second only", value: types.StringValue("second_abc"), wantErr: false},
		{name: "matches none", value: types.StringValue("third_abc"), wantErr: true},
		{name: "null skipped", value: types.StringNull(), wantErr: false},
		{name: "unknown skipped", value: types.StringUnknown(), wantErr: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			RegexMatches(regexps, "").ValidateString(context.Background(), validator.StringRequest{
				Path:        path.Root("test"),
				ConfigValue: tc.value,
			}, resp)
			if resp.Diagnostics.HasError() != tc.wantErr {
				t.Errorf("value %v: HasError() = %v, want %v (diags: %v)",
					tc.value, resp.Diagnostics.HasError(), tc.wantErr, resp.Diagnostics)
			}
		})
	}
}
