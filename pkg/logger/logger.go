// Package logger provides a structured logger built on top of Uber's Zap library.
//
// WHY ZAP?
// Standard library log is fine for simple scripts, but in production banking
// systems you need:
//   - Structured fields (JSON output that log aggregators can query)
//   - Log levels (suppress debug logs in production)
//   - Performance (Zap is one of the fastest loggers in Go)
//
// TWO MODES:
//   - Development (console): Human-readable colored output for your terminal
//   - Production (json): Machine-readable JSON for log aggregation tools
//     like Elasticsearch, Loki, or CloudWatch
//
// USAGE:
//
//	logger.Info("application started", zap.Int("port", 8080))
//	logger.Error("database connection failed", zap.Error(err))
//	logger.With(zap.String("application_id", id)).Info("step saved")
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger so we can extend it with helper methods if needed.
type Logger struct {
	*zap.Logger
}

// New creates and configures a new Logger based on the provided settings.
//
// Parameters:
//   - level:  "debug" | "info" | "warn" | "error"
//   - format: "console" (human-readable) | "json" (structured)
//   - output: "stdout" or an absolute file path
func New(level, format, output string) (*Logger, error) {
	// ── Parse log level ───────────────────────────────────────────────────────
	// zapcore.Level is a numeric type. ParseLevel converts "debug" → Level(-1), etc.
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel // Default to info if invalid level provided
	}

	// ── Configure the encoder ─────────────────────────────────────────────────
	// The encoder decides how each log line looks.
	var encoderCfg zapcore.EncoderConfig
	if format == "console" {
		// Development: colored, readable timestamps like "2024-01-15T10:30:45.123+0700"
		encoderCfg = zap.NewDevelopmentEncoderConfig()
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		// Production: ISO timestamps, lowercase level names for JSON parsers
		encoderCfg = zap.NewProductionEncoderConfig()
		encoderCfg.TimeKey = "timestamp"
		encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	// ── Configure the output writer ───────────────────────────────────────────
	var writeSyncer zapcore.WriteSyncer
	if output == "stdout" || output == "" {
		writeSyncer = zapcore.AddSync(os.Stdout)
	} else {
		// Write to file — useful for production servers
		// The file is opened in append mode so logs survive restarts
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			// Fall back to stdout if we can't open the file
			writeSyncer = zapcore.AddSync(os.Stdout)
		} else {
			// Tee to both file AND stdout so you can see logs in docker logs too
			writeSyncer = zapcore.NewMultiWriteSyncer(
				zapcore.AddSync(f),
				zapcore.AddSync(os.Stdout),
			)
		}
	}

	// ── Build the encoder ─────────────────────────────────────────────────────
	var encoder zapcore.Encoder
	if format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	}

	// ── Assemble the core ─────────────────────────────────────────────────────
	// zapcore.NewCore wires together: what to log + how to format + where to write
	core := zapcore.NewCore(encoder, writeSyncer, zapLevel)

	// ── Build the final logger ────────────────────────────────────────────────
	// zap.AddCaller() adds filename:line to every log entry — critical for debugging
	// zap.AddStacktrace() adds stack trace for Error level and above
	zapLogger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(0),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return &Logger{zapLogger}, nil
}

// With creates a child logger with pre-attached fields.
// Use this to create a request-scoped logger that always includes the
// application_id or request_id without repeating it in every log call.
//
// Example:
//
//	reqLogger := logger.With(
//	    zap.String("request_id", requestID),
//	    zap.String("application_id", appID),
//	)
//	reqLogger.Info("processing step 3")  // automatically includes both fields
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{l.Logger.With(fields...)}
}

// Sync flushes any buffered log entries.
// IMPORTANT: Call this with defer in main() to ensure all logs are written
// before the program exits.
//
//	defer logger.Sync()
func (l *Logger) Sync() {
	// We intentionally ignore the error here because on some systems
	// (notably when writing to stdout/stderr) Sync() returns EINVAL.
	// This is a known Zap issue and doesn't indicate a real problem.
	_ = l.Logger.Sync()
}
