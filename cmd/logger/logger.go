package logger

import (
	"io"
	"log/slog"
	"os"
	"time"
)

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
}

// LogLevel represents the log level
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// Config holds logger configuration
type Config struct {
	Level  LogLevel `toml:"level"`
	Format string   `toml:"format"` // "json" or "text"
	Output string   `toml:"output"` // "stdout", "stderr", or file path
}

// NewFromConfigStruct creates a logger from a config struct with string level
func NewFromConfigStruct(level, format, output string) *Logger {
	config := &Config{
		Level:  LogLevel(level),
		Format: format,
		Output: output,
	}
	return New(config)
}

// DefaultConfig returns default logger configuration
func DefaultConfig() *Config {
	return &Config{
		Level:  LevelInfo,
		Format: "json",
		Output: "stdout",
	}
}

// New creates a new logger instance
func New(config *Config) *Logger {
	if config == nil {
		config = DefaultConfig()
	}

	// Determine log level
	var level slog.Level
	switch config.Level {
	case LevelDebug:
		level = slog.LevelDebug
	case LevelInfo:
		level = slog.LevelInfo
	case LevelWarn:
		level = slog.LevelWarn
	case LevelError:
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Determine output destination
	var output io.Writer
	switch config.Output {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		// Try to open file, fallback to stdout on error
		if file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640); err == nil {
			output = file
		} else {
			output = os.Stdout
		}
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize timestamp format
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "timestamp",
					Value: slog.StringValue(a.Value.Time().Format(time.RFC3339)),
				}
			}
			return a
		},
	}

	// Create appropriate handler based on format
	var handler slog.Handler
	if config.Format == "text" {
		handler = slog.NewTextHandler(output, opts)
	} else {
		handler = slog.NewJSONHandler(output, opts)
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

// Global logger instance
var defaultLogger *Logger

// Init initializes the global logger
func Init(config *Config) {
	defaultLogger = New(config)
}

// Default returns the default logger, creating one if it doesn't exist
func Default() *Logger {
	if defaultLogger == nil {
		defaultLogger = New(DefaultConfig())
	}
	return defaultLogger
}

// Convenience functions that use the default logger

// Debug logs a debug message
func Debug(msg string, args ...any) {
	Default().Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	Default().Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	Default().Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	Default().Error(msg, args...)
}

// With returns a logger with additional context
func With(args ...any) *Logger {
	return &Logger{Logger: Default().With(args...)}
}

// LogRequest logs an HTTP request with structured data
func (l *Logger) LogRequest(method, path, userAgent string, duration time.Duration, statusCode int, nodeUsed string) {
	l.Info("request completed",
		slog.String("method", method),
		slog.String("path", path),
		slog.String("user_agent", userAgent),
		slog.String("duration", duration.String()),
		slog.Int("status_code", statusCode),
		slog.String("node_used", nodeUsed),
	)
}

// LogStartup logs application startup
func (l *Logger) LogStartup(port int, configFile string, nodeCount int) {
	l.Info("beacon proxy starting",
		slog.Int("port", port),
		slog.String("config_file", configFile),
		slog.Int("node_count", nodeCount),
		slog.String("version", "dev"),
	)
}

// LogError logs errors with context
func (l *Logger) LogError(operation string, err error, context ...any) {
	args := []any{
		slog.String("operation", operation),
		slog.String("error", err.Error()),
	}
	args = append(args, context...)
	l.Error("operation failed", args...)
}

// LogConfig logs configuration loading
func (l *Logger) LogConfig(configPath string, loadedViaEnv bool, nodeCount int) {
	l.Info("configuration loaded",
		slog.String("config_path", configPath),
		slog.Bool("loaded_via_env", loadedViaEnv),
		slog.Int("node_count", nodeCount),
	)
}
