package observability

import (
	"context"
	"log/slog"
	"os"
)

type Logger struct {
	*slog.Logger
}

func NewLogger(serviceName string) *Logger {
	// Default to JSON for production-grade structured logging
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler).With("service", serviceName)
	return &Logger{logger}
}

// WithContext adds trace information from context if available
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// In a real implementation, we would extract trace ID from ctx
	return l
}
