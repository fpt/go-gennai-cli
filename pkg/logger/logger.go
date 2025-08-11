package logger

import (
	"log/slog"
	"os"
)

// LogLevel represents the available log levels
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// Logger provides a structured logger instance configured for the application
type Logger struct {
	*slog.Logger
}

// NewLogger creates a new structured logger with the specified level
func NewLogger(level LogLevel) *Logger {
	var slogLevel slog.Level

	switch level {
	case LogLevelDebug:
		slogLevel = slog.LevelDebug
	case LogLevelInfo:
		slogLevel = slog.LevelInfo
	case LogLevelWarn:
		slogLevel = slog.LevelWarn
	case LogLevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo // Default to info
	}

	// Create a text handler with custom options
	opts := &slog.HandlerOptions{
		Level: slogLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize time format for readability
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "time",
					Value: slog.StringValue(a.Value.Time().Format("15:04:05")),
				}
			}
			return a
		},
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	logger := slog.New(handler)

	return &Logger{Logger: logger}
}

// NewDefaultLogger creates a logger with INFO level for general use
func NewDefaultLogger() *Logger {
	return NewLogger(LogLevelInfo)
}

// NewDebugLogger creates a logger with DEBUG level for development
func NewDebugLogger() *Logger {
	return NewLogger(LogLevelDebug)
}

// WithComponent creates a logger with a component context for better tracing
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With("component", component),
	}
}

// WithSession creates a logger with session context for request tracing
func (l *Logger) WithSession(sessionID string) *Logger {
	return &Logger{
		Logger: l.Logger.With("session", sessionID),
	}
}

// InfoWithIcon logs info message with emoji for user-friendly output
func (l *Logger) InfoWithIcon(icon string, msg string, args ...any) {
	l.Info(icon+" "+msg, args...)
}

// WarnWithIcon logs warning message with emoji for user-friendly output
func (l *Logger) WarnWithIcon(icon string, msg string, args ...any) {
	l.Warn(icon+" "+msg, args...)
}

// ErrorWithIcon logs error message with emoji for user-friendly output
func (l *Logger) ErrorWithIcon(icon string, msg string, args ...any) {
	l.Error(icon+" "+msg, args...)
}

// DebugWithIcon logs debug message with emoji for development
func (l *Logger) DebugWithIcon(icon string, msg string, args ...any) {
	l.Debug(icon+" "+msg, args...)
}

// Default logger instance - single instance for the entire application
var Default = NewDefaultLogger()

// SetGlobalLogLevel updates the global default logger with a new log level
// This affects all component loggers created after this call
func SetGlobalLogLevel(level LogLevel) {
	Default = NewLogger(level)
}

// NewComponentLogger creates a new logger for a specific component
func NewComponentLogger(component string) *Logger {
	return Default.WithComponent(component)
}
