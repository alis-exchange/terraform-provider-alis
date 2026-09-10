package spanner

import (
	"strings"
	"testing"

	"terraform-provider-alis/internal/spanner/services"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func TestConfigureSpannerService(t *testing.T) {
	t.Run("nil provider data is a silent no-op", func(t *testing.T) {
		var diags diag.Diagnostics
		service, ok := configureSpannerService(nil, &diags)
		if ok || service != nil || diags.HasError() {
			t.Errorf("got (%v, %v, errs=%v), want silent (nil, false)", service, ok, diags)
		}
	})

	t.Run("wrong type produces diagnostic naming the actual expected type", func(t *testing.T) {
		var diags diag.Diagnostics
		service, ok := configureSpannerService("not-a-service", &diags)
		if ok || service != nil || !diags.HasError() {
			t.Fatalf("got (%v, %v, errs=%v), want error diagnostic", service, ok, diags)
		}
		detail := diags.Errors()[0].Detail()
		if !strings.Contains(detail, "services.SpannerService") {
			t.Errorf("diagnostic %q must name services.SpannerService", detail)
		}
	})

	t.Run("correct type is returned", func(t *testing.T) {
		var diags diag.Diagnostics
		want := &services.SpannerService{}
		service, ok := configureSpannerService(want, &diags)
		if !ok || service != want || diags.HasError() {
			t.Errorf("got (%v, %v, errs=%v), want the service back", service, ok, diags)
		}
	})
}
