package spanner

import (
	"strings"
	"testing"

	"terraform-provider-alis/internal"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func TestConfigureProviderConfig(t *testing.T) {
	t.Run("nil provider data is a silent no-op", func(t *testing.T) {
		var diags diag.Diagnostics
		config, ok := configureProviderConfig(nil, &diags)
		if ok || config != nil || diags.HasError() {
			t.Errorf("got (%v, %v, errs=%v), want silent (nil, false)", config, ok, diags)
		}
	})

	t.Run("wrong type produces diagnostic naming the actual expected type", func(t *testing.T) {
		var diags diag.Diagnostics
		config, ok := configureProviderConfig("not-a-config", &diags)
		if ok || config != nil || !diags.HasError() {
			t.Fatalf("got (%v, %v, errs=%v), want error diagnostic", config, ok, diags)
		}
		detail := diags.Errors()[0].Detail()
		if !strings.Contains(detail, "internal.ProviderConfig") {
			t.Errorf("diagnostic %q must name internal.ProviderConfig", detail)
		}
	})

	t.Run("correct type is returned", func(t *testing.T) {
		var diags diag.Diagnostics
		want := &internal.ProviderConfig{GoogleProjectId: "p"}
		config, ok := configureProviderConfig(want, &diags)
		if !ok || config != want || diags.HasError() {
			t.Errorf("got (%v, %v, errs=%v), want the config back", config, ok, diags)
		}
	})
}
