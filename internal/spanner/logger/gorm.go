package logger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

var (
	// Discard sends every log message to io.Discard.
	Discard = New(log.New(io.Discard, "", log.LstdFlags), logger.Config{})
	// Default writes to stdout at Warn level, with color and a 200ms
	// slow-SQL threshold.
	Default = New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: false,
		Colorful:                  true,
	})
	// Recorder captures the most recent traced SQL statement instead of
	// printing it, delegating all other logging to Default.
	Recorder = traceRecorder{Interface: Default, BeginAt: time.Now()}
)

// New builds a gorm logger that writes formatted messages via writer and
// mirrors each one to tflog. config.Colorful selects ANSI-colored formats.
func New(writer logger.Writer, config logger.Config) logger.Interface {
	var (
		infoStr      = "%s\n[info] "
		warnStr      = "%s\n[warn] "
		errStr       = "%s\n[error] "
		traceStr     = "%s\n[%.3fms] [rows:%v] %s"
		traceWarnStr = "%s %s\n[%.3fms] [rows:%v] %s"
		traceErrStr  = "%s %s\n[%.3fms] [rows:%v] %s"
	)

	if config.Colorful {
		infoStr = logger.Green + "%s\n" + logger.Reset + logger.Green + "[info] " + logger.Reset
		warnStr = logger.BlueBold + "%s\n" + logger.Reset + logger.Magenta + "[warn] " + logger.Reset
		errStr = logger.Magenta + "%s\n" + logger.Reset + logger.Red + "[error] " + logger.Reset
		traceStr = logger.Green + "%s\n" + logger.Reset + logger.Yellow + "[%.3fms] " + logger.BlueBold + "[rows:%v]" + logger.Reset + " %s"
		traceWarnStr = logger.Green + "%s " + logger.Yellow + "%s\n" + logger.Reset + logger.RedBold + "[%.3fms] " + logger.Yellow + "[rows:%v]" + logger.Magenta + " %s" + logger.Reset
		traceErrStr = logger.RedBold + "%s " + logger.MagentaBold + "%s\n" + logger.Reset + logger.Yellow + "[%.3fms] " + logger.BlueBold + "[rows:%v]" + logger.Reset + " %s"
	}

	return &tfLogger{
		Writer:       writer,
		Config:       config,
		infoStr:      infoStr,
		warnStr:      warnStr,
		errStr:       errStr,
		traceStr:     traceStr,
		traceWarnStr: traceWarnStr,
		traceErrStr:  traceErrStr,
	}
}

// tfLogger implements gorm's logger.Interface, pairing gorm's standard
// formatting (via the embedded Writer) with a tflog mirror of each message.
type tfLogger struct {
	logger.Writer
	logger.Config
	infoStr, warnStr, errStr            string
	traceStr, traceErrStr, traceWarnStr string
}

// LogMode returns a copy of the logger at the given level; the receiver is
// unchanged.
func (l *tfLogger) LogMode(level logger.LogLevel) logger.Interface {
	newlogger := *l
	newlogger.LogLevel = level
	return &newlogger
}

// Info logs an info-level message to the writer and mirrors it to tflog.
func (l *tfLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		tflog.Info(ctx, render(msg, data...), callerField())
		l.Printf(l.infoStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

// Warn logs a warn-level message to the writer and mirrors it to tflog.
func (l *tfLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		tflog.Warn(ctx, render(msg, data...), callerField())
		l.Printf(l.warnStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

// Error logs an error-level message to the writer and mirrors it to tflog.
func (l *tfLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		tflog.Error(ctx, render(msg, data...), callerField())
		l.Printf(l.errStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

// render fills gorm's format string with its arguments. Terraform renders log
// messages verbatim, so the format string itself must never be the message —
// it would reach operators with its verbs and color codes unsubstituted.
func render(msg string, data ...interface{}) string {
	if len(data) == 0 {
		return msg
	}

	return fmt.Sprintf(msg, data...)
}

// callerField carries the gorm call site that the writer's format prefix
// carries for the terminal.
func callerField() map[string]interface{} {
	return map[string]interface{}{"caller": utils.FileWithLineNum()}
}

// Trace logs a completed SQL statement with its duration and row count,
// picking the error, slow-query, or info format, and mirrors the entry to
// tflog with structured fields.
//
//nolint:cyclop // log-format selection is one flat switch over level and latency; splitting it would obscure the flow
func (l *tfLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= logger.Error && (!errors.Is(err, logger.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		if rows == -1 {
			l.Printf(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			l.Printf(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
		tflog.Trace(ctx, sql, map[string]interface{}{
			"caller":       utils.FileWithLineNum(),
			"rowsAffected": rows,
			"error":        err,
			"duration":     elapsed,
		})
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= logger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		if rows == -1 {
			l.Printf(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			l.Printf(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
		tflog.Trace(ctx, sql, map[string]interface{}{
			"caller":       utils.FileWithLineNum(),
			"rowsAffected": rows,
			"warning":      slowLog,
			"duration":     elapsed,
		})
	case l.LogLevel == logger.Info:
		sql, rows := fc()
		if rows == -1 {
			l.Printf(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			l.Printf(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
		tflog.Trace(ctx, sql, map[string]interface{}{
			"caller":       utils.FileWithLineNum(),
			"rowsAffected": rows,
			"duration":     elapsed,
		})
	}
}

// ParamsFilter implements gorm's params-filter hook: with
// ParameterizedQueries set it strips the bound params so parameter values
// never reach log output.
func (l *tfLogger) ParamsFilter(ctx context.Context, sql string, params ...interface{}) (string, []interface{}) {
	if l.ParameterizedQueries {
		return sql, nil
	}
	return sql, params
}

// traceRecorder overrides Trace to capture the last SQL statement, row count,
// and error for inspection instead of printing them; every other method
// delegates to the embedded Interface.
type traceRecorder struct {
	logger.Interface
	BeginAt      time.Time
	SQL          string
	RowsAffected int64
	Err          error
}

// New returns a fresh recorder with the same delegate Interface and no
// captured trace.
func (l *traceRecorder) New() *traceRecorder {
	return &traceRecorder{Interface: l.Interface, BeginAt: time.Now()}
}

// Trace records the statement, row count, and error rather than logging them.
func (l *traceRecorder) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.BeginAt = begin
	l.SQL, l.RowsAffected = fc()
	l.Err = err
}
