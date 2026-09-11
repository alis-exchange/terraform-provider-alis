package services

import (
	"terraform-provider-alis/internal/spanner/conn"
	"terraform-provider-alis/internal/utils"
)

// SpannerService talks to Spanner exclusively through the Connection module —
// no client construction, logging, credential, or retry concerns live here.
type SpannerService struct {
	conn conn.Connection
	// blobs reads gs:// descriptor-set sources; nil means such sources are
	// rejected with a clear error rather than attempted with ADC.
	blobs utils.BlobReader
}

// Option configures optional collaborators on a SpannerService.
type Option func(*SpannerService)

// WithBlobReader enables gs:// sources for proto bundle descriptor sets.
func WithBlobReader(r utils.BlobReader) Option {
	return func(s *SpannerService) { s.blobs = r }
}

// NewSpannerService returns a SpannerService that performs all Spanner work
// over cn.
func NewSpannerService(cn conn.Connection, opts ...Option) *SpannerService {
	s := &SpannerService{conn: cn}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
