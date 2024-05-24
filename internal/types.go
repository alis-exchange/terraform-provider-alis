package internal

import (
	bigtableservices "terraform-provider-alis/internal/bigtable/services"
	discoveryengineservices "terraform-provider-alis/internal/discoveryengine/services"
	spannerservices "terraform-provider-alis/internal/spanner/services"
)

type ProviderConfig struct {
	GoogleProjectId        string
	BigtableService        *bigtableservices.BigtableService
	SpannerService         *spannerservices.SpannerService
	DiscoveryEngineService *discoveryengineservices.DiscoveryEngineService
}
