package services

import (
	"log"
	"os"
	"time"

	customloggers "terraform-provider-alis/internal/spanner/logger"

	_ "github.com/googleapis/go-sql-spanner"
	googleoauth "golang.org/x/oauth2/google"
	"gorm.io/gorm/logger"
)

var tfLogger = customloggers.New(
	log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
	logger.Config{
		SlowThreshold:             200 * time.Millisecond, // Slow SQL threshold
		LogLevel:                  logger.Info,            // Log level
		IgnoreRecordNotFoundError: false,                  // Ignore ErrRecordNotFound error for logger
		ParameterizedQueries:      true,                   // Don't include params in the SQL log
		Colorful:                  true,                   // Disable color
	},
)

const (
	DatabaseDialect_GoogleStandardSQL = "GOOGLE_STANDARD_SQL"
	DatabaseDialect_PostgreSQL        = "POSTGRESQL"

	DatabaseState_Creating        = "CREATING"
	DatabaseState_Ready           = "READY"
	DatabaseState_ReadyOptimizing = "READY_OPTIMIZING"

	DatabaseEncryptionType_CustomerManaged         = "CUSTOMER_MANAGED_ENCRYPTION"
	DatabaseEncryptionType_GoogleDefaultEncryption = "GOOGLE_DEFAULT_ENCRYPTION"
)

type SpannerService struct {
	GoogleCredentials *googleoauth.Credentials
}

func NewSpannerService(creds *googleoauth.Credentials) *SpannerService {
	return &SpannerService{
		GoogleCredentials: creds,
	}
}
