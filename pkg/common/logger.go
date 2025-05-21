package common

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// Logger represents our custom logger
type Logger struct {
	*log.Logger
	serviceName string
}

// LogLevel represents the severity of a log message
type LogLevel int

const (
	// DEBUG level for detailed information
	DEBUG LogLevel = iota
	// INFO level for general operational information
	INFO
	// WARN level for warning messages
	WARN
	// ERROR level for error messages
	ERROR
	// FATAL level for fatal messages that will terminate the application
	FATAL
)

// String returns the string representation of a LogLevel
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// NewLogger creates a new logger instance
func NewLogger(serviceName string) *Logger {
	return &Logger{
		Logger:      log.New(os.Stdout, "", 0),
		serviceName: serviceName,
	}
}

// log formats and writes a log message
func (l *Logger) log(level LogLevel, format string, v ...interface{}) {
	// Get caller information
	_, file, line, _ := runtime.Caller(2)
	// Extract just the file name from the full path
	parts := strings.Split(file, "/")
	file = parts[len(parts)-1]

	// Format the message
	msg := fmt.Sprintf(format, v...)

	// Create the log entry
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z")
	logEntry := fmt.Sprintf("[%s] [%s] [%s] [%s:%d] %s",
		timestamp,
		level.String(),
		l.serviceName,
		file,
		line,
		msg,
	)

	// Write the log entry
	l.Println(logEntry)

	// If this is a fatal message, terminate the application
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	l.log(DEBUG, format, v...)
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	l.log(INFO, format, v...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	l.log(WARN, format, v...)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	l.log(ERROR, format, v...)
}

// InfoCtx logs an info message, expecting a context.Context as the first argument (though not directly used in this basic version yet).
// It's added for API consistency with future context-aware logging features.
func (l *Logger) InfoCtx(ctx any, format string, v ...interface{}) {
	// Temporarily adjust caller depth for context methods
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
	}
	parts := strings.Split(file, "/")
	fileName := parts[len(parts)-1]

	msg := fmt.Sprintf(format, v...)
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z")
	logEntry := fmt.Sprintf("[%s] [%s] [%s] [%s:%d] %s (ctx)", // Added (ctx) for now
		timestamp,
		INFO.String(),
		l.serviceName,
		fileName,
		line,
		msg,
	)
	l.Println(logEntry)
}

// ErrorCtx logs an error message, expecting a context.Context as the first argument.
func (l *Logger) ErrorCtx(ctx any, format string, v ...interface{}) {
	// Temporarily adjust caller depth for context methods
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
	}
	parts := strings.Split(file, "/")
	fileName := parts[len(parts)-1]

	msg := fmt.Sprintf(format, v...)
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z")
	logEntry := fmt.Sprintf("[%s] [%s] [%s] [%s:%d] %s (ctx)", // Added (ctx) for now
		timestamp,
		ERROR.String(),
		l.serviceName,
		fileName,
		line,
		msg,
	)
	l.Println(logEntry)
}

// Fatal logs a fatal message and terminates the application
func (l *Logger) Fatal(format string, v ...interface{}) {
	l.log(FATAL, format, v...)
}

// LogRequest logs an HTTP request
func (l *Logger) LogRequest(method, path, remoteAddr, userAgent string, statusCode int, latency time.Duration) {
	l.Info("Request: method=%s path=%s remote_addr=%s user_agent=%s status=%d latency=%s",
		method, path, remoteAddr, userAgent, statusCode, latency)
}

// LogError logs an error with context
func (l *Logger) LogError(err error, context string) {
	l.Error("Error: %s - %v", context, err)
}

// LogPanic logs a panic with its stack trace
func (l *Logger) LogPanic(r interface{}) {
	buf := make([]byte, 1024)
	n := runtime.Stack(buf, false)
	l.Error("Panic: %v\nStack: %s", r, string(buf[:n]))
}
