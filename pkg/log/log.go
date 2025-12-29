package log

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"sync"
	"time"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	defaultLoggerOnce sync.Once
	defaultLogger     *zap.Logger
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	OperationKey contextKey = "operation"
)

func Default() *zap.Logger {
	defaultLoggerOnce.Do(func() {
		if defaultLogger == nil {
			ec := zap.NewProductionEncoderConfig()
			ec.EncodeTime = zapcore.TimeEncoderOfLayout(time.DateTime)
			ec.EncodeLevel = zapcore.CapitalColorLevelEncoder

			var ws zapcore.WriteSyncer
			var wss []zapcore.WriteSyncer

			// wss = append(wss, zapcore.AddSync(os.Stdout))
			wss = append(wss, zapcore.AddSync(os.Stderr))
			ws = zapcore.NewMultiWriteSyncer(wss...)

			enc := zapcore.NewConsoleEncoder(ec)
			var level zapcore.Level
			if flag.Debug {
				level = zapcore.DebugLevel
			} else {
				level = zapcore.InfoLevel
			}
			core := zapcore.NewCore(enc, ws, level)
			options := []zap.Option{
				zap.AddStacktrace(zapcore.ErrorLevel),
				zap.AddCaller(),
				zap.AddCallerSkip(1),
			}
			defaultLogger = zap.New(core, options...)
		}
	})

	return defaultLogger
}

func SetDefault(logger *zap.Logger) {
	if logger != nil {
		defaultLogger = logger
	}
}

func Logger() *zap.Logger {
	return defaultLogger
}

func Debug(msg string, fields ...zap.Field) {
	Default().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	Default().Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	Default().Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	Default().Error(msg, fields...)
}

func Panic(msg string, fields ...zap.Field) {
	Default().Panic(msg, fields...)
}

func Debugf(format string, args ...any) {
	Default().Sugar().Debugf(format, args...)
}

func Infof(format string, args ...any) {
	Default().Sugar().Infof(format, args...)
}

func Warnf(format string, args ...any) {
	Default().Sugar().Warnf(format, args...)
}

func Errorf(format string, args ...any) {
	Default().Sugar().Errorf(format, args...)
}

func Fatal(msg string, fields ...zap.Field) {
	Default().Fatal(msg, fields...)
}

func Fatalf(format string, args ...any) {
	Default().Sugar().Fatalf(format, args...)
}

// GenerateRequestID creates a new random request ID
func GenerateRequestID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithOperation adds an operation name to the context
func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, OperationKey, operation)
}

// GetContextFields extracts logging fields from context
func GetContextFields(ctx context.Context) []zap.Field {
	var fields []zap.Field

	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}

	if operation, ok := ctx.Value(OperationKey).(string); ok && operation != "" {
		fields = append(fields, zap.String("operation", operation))
	}

	return fields
}

// SanitizeURL removes sensitive information from URLs for logging
func SanitizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "[invalid-url]"
	}

	// Remove query parameters that might contain tokens
	if parsedURL.RawQuery != "" {
		parsedURL.RawQuery = "[redacted]"
	}

	// Remove user info (if any)
	parsedURL.User = nil

	return parsedURL.String()
}

// Context-aware logging functions
func DebugCtx(ctx context.Context, msg string, fields ...zap.Field) {
	fields = append(GetContextFields(ctx), fields...)
	Default().Debug(msg, fields...)
}

func InfoCtx(ctx context.Context, msg string, fields ...zap.Field) {
	fields = append(GetContextFields(ctx), fields...)
	Default().Info(msg, fields...)
}

func WarnCtx(ctx context.Context, msg string, fields ...zap.Field) {
	fields = append(GetContextFields(ctx), fields...)
	Default().Warn(msg, fields...)
}

func ErrorCtx(ctx context.Context, msg string, fields ...zap.Field) {
	fields = append(GetContextFields(ctx), fields...)
	Default().Error(msg, fields...)
}

func DebugfCtx(ctx context.Context, format string, args ...any) {
	fields := GetContextFields(ctx)
	Default().With(fields...).Sugar().Debugf(format, args...)
}

func InfofCtx(ctx context.Context, format string, args ...any) {
	fields := GetContextFields(ctx)
	Default().With(fields...).Sugar().Infof(format, args...)
}

func WarnfCtx(ctx context.Context, format string, args ...any) {
	fields := GetContextFields(ctx)
	Default().With(fields...).Sugar().Warnf(format, args...)
}

func ErrorfCtx(ctx context.Context, format string, args ...any) {
	fields := GetContextFields(ctx)
	Default().With(fields...).Sugar().Errorf(format, args...)
}

// Helper functions for common field types
func StringField(key, value string) zap.Field {
	return zap.String(key, value)
}

func IntField(key string, value int) zap.Field {
	return zap.Int(key, value)
}

func BoolField(key string, value bool) zap.Field {
	return zap.Bool(key, value)
}

func DurationField(key string, value time.Duration) zap.Field {
	return zap.Duration(key, value)
}

func ErrorField(err error) zap.Field {
	return zap.Error(err)
}

func SanitizedURLField(key, rawURL string) zap.Field {
	return zap.String(key, SanitizeURL(rawURL))
}

// WithMCPContext creates a new context with request ID and operation for MCP tool logging
func WithMCPContext(ctx context.Context, toolName string) (context.Context, string) {
	requestID := GenerateRequestID()
	ctx = WithRequestID(ctx, requestID)
	ctx = WithOperation(ctx, toolName)
	return ctx, requestID
}

// LogMCPToolStart logs the start of an MCP tool execution
func LogMCPToolStart(ctx context.Context, toolName string, params map[string]interface{}) {
	fields := []zap.Field{
		StringField("tool", toolName),
	}

	// Add sanitized parameters (be careful not to log sensitive data)
	for key, value := range params {
		if key == "token" || key == "password" || key == "secret" {
			fields = append(fields, StringField(key, "[redacted]"))
		} else {
			fields = append(fields, zap.Any(key, value))
		}
	}

	InfoCtx(ctx, "Starting MCP tool execution", fields...)
}

// LogMCPToolComplete logs successful completion of an MCP tool execution
func LogMCPToolComplete(ctx context.Context, toolName string, duration time.Duration, resultSummary string) {
	InfoCtx(ctx, "MCP tool execution completed successfully",
		StringField("tool", toolName),
		DurationField("total_duration", duration),
		StringField("result_summary", resultSummary),
	)
}

// LogMCPToolError logs failed MCP tool execution
func LogMCPToolError(ctx context.Context, toolName string, duration time.Duration, err error) {
	ErrorCtx(ctx, "MCP tool execution failed",
		StringField("tool", toolName),
		DurationField("total_duration", duration),
		ErrorField(err),
	)
}
