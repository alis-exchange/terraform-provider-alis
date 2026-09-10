package spanner

import (
	"fmt"

	"terraform-provider-alis/internal/spanner/services"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// configureSpannerService extracts the Spanner service shared by every resource
// and data source Configure method. nil provider data is a silent no-op —
// Terraform calls Configure before the provider is configured. ok is false
// whenever the service is unusable; a diagnostic is added only for the
// wrong-type case.
func configureSpannerService(providerData any, diags *diag.Diagnostics) (*services.SpannerService, bool) {
	if providerData == nil {
		return nil, false
	}

	service, ok := providerData.(*services.SpannerService)
	if !ok {
		diags.AddError(
			"Unexpected Provider Configure Type",
			fmt.Sprintf("Expected *services.SpannerService, got: %T. Please report this issue to the provider developers.", providerData),
		)
		return nil, false
	}

	return service, true
}
