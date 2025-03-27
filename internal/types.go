package internal

import (
	spannerservices "terraform-provider-alis/internal/spanner/services"
)

type ProviderConfig struct {
	GoogleProjectId string
	SpannerService  *spannerservices.SpannerService
}
