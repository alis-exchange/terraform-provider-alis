package spanner

import (
	"fmt"

	"terraform-provider-alis/internal"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// configureProviderConfig extracts the provider configuration shared by every
// resource and data source Configure method. nil provider
// data is a silent no-op — Terraform calls Configure before the provider is
// configured. ok is false whenever config is unusable; a diagnostic is added
// only for the wrong-type case.
func configureProviderConfig(providerData any, diags *diag.Diagnostics) (*internal.ProviderConfig, bool) {
	if providerData == nil {
		return nil, false
	}

	config, ok := providerData.(*internal.ProviderConfig)
	if !ok {
		diags.AddError(
			"Unexpected Provider Configure Type",
			fmt.Sprintf("Expected *internal.ProviderConfig, got: %T. Please report this issue to the provider developers.", providerData),
		)
		return nil, false
	}

	return config, true
}
