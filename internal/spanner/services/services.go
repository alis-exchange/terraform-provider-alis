package services

import (
	"terraform-provider-alis/internal/spanner/conn"
)

// Database dialect, state, and encryption values as reported by the Spanner
// Database Admin API.
const (
	DatabaseDialect_GoogleStandardSQL = "GOOGLE_STANDARD_SQL"
	DatabaseDialect_PostgreSQL        = "POSTGRESQL"

	DatabaseState_Creating        = "CREATING"
	DatabaseState_Ready           = "READY"
	DatabaseState_ReadyOptimizing = "READY_OPTIMIZING"

	DatabaseEncryptionType_CustomerManaged         = "CUSTOMER_MANAGED_ENCRYPTION"
	DatabaseEncryptionType_GoogleDefaultEncryption = "GOOGLE_DEFAULT_ENCRYPTION"
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
