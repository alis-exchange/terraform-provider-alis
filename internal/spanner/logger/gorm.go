package logger

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gorm.io/gorm/logger"
)

func New(level logger.LogLevel) logger.Interface {
	return &tfLogger{
		LogLevel: level,
	}
}

type tfLogger struct {
	LogLevel logger.LogLevel
}

// LogMode log mode
func (l *tfLogger) LogMode(level logger.LogLevel) logger.Interface {
	newlogger := *l
	newlogger.LogLevel = level
	return &newlogger
}

// Info print info
func (l *tfLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		tflog.Info(ctx, msg)
	}
}

// Warn print warn messages
func (l *tfLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		tflog.Warn(ctx, msg)
	}
}

// Error print error messages
func (l *tfLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		tflog.Error(ctx, msg)
	}
}

// Trace print sql message
func (l *tfLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	sql, rowsAffected := fc()
	tflog.Trace(ctx, sql, map[string]interface{}{
		"rowsAffected": rowsAffected,
		"error":        err,
		"duration":     time.Since(begin),
	})
}
