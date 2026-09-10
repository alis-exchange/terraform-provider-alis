package services

import (
	"terraform-provider-alis/internal/spanner/conn"
)

// SpannerService talks to Spanner exclusively through the Connection module —
// no client construction, logging, credential, or retry concerns live here.
type SpannerService struct {
	conn conn.Connection
}

// NewSpannerService returns a SpannerService that performs all Spanner work
// over cn.
func NewSpannerService(cn conn.Connection) *SpannerService {
	return &SpannerService{conn: cn}
}
