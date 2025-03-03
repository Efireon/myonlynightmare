package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

// Log levels
const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// Logger handles logging functionalities
type Logger struct {
	level     LogLevel
	logger    *log.Logger
	file      *os.File
	useColors bool
}

// levelColors maps log levels to ANSI color codes
var levelColors = map[LogLevel]string{
	DEBUG: "\033[36m", // Cyan
	INFO:  "\033[32m", // Green
	WARN:  "\033[33m", // Yellow
	ERROR: "\033[31m", // Red
	FATAL: "\033[35m", // Magenta
}

// levelPrefixes maps log levels to text prefixes
var levelPrefixes = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO ",
	WARN:  "WARN ",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

// NewLogger creates a new logger with the specified log level
func NewLogger(levelStr string) *Logger {
	var level LogLevel

	// Parse log level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = DEBUG
	case "info":
		level = INFO
	case "warn":
		level = WARN
	case "error":
		level = ERROR
	case "fatal":
		level = FATAL
	default:
		level = INFO // Default to INFO
	}

	// Create logger
	logger := &Logger{
		level:     level,
		logger:    log.New(os.Stdout, "", 0), // We'll format the prefix manually
		useColors: true,                      // Enable colors by default
	}

	// Disable colors if not in a terminal
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		logger.useColors = false
	}

	return logger
}

// NewFileLogger creates a new logger that writes to a file
func NewFileLogger(levelStr, filePath string) (*Logger, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Open log file
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	// Create basic logger
	logger := NewLogger(levelStr)

	// Set output to file and disable colors
	logger.logger.SetOutput(file)
	logger.file = file
	logger.useColors = false

	return logger, nil
}

// NewMultiLogger creates a logger that writes to both console and file
func NewMultiLogger(levelStr, filePath string) (*Logger, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Open log file
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	// Create basic logger
	logger := NewLogger(levelStr)

	// Set output to both console and file
	multi := io.MultiWriter(os.Stdout, file)
	logger.logger.SetOutput(multi)
	logger.file = file

	return logger, nil
}

// log logs a message with the specified level
func (l *Logger) log(level LogLevel, v ...interface{}) {
	if level < l.level {
		return
	}

	// Get caller info
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "unknown"
		line = 0
	}

	// Format the file path to just the file name
	file = filepath.Base(file)

	// Get the current time
	now := time.Now().Format("2006/01/02 15:04:05")

	// Format the log prefix
	prefix := fmt.Sprintf("%s [%s] %s:%d:", now, levelPrefixes[level], file, line)

	// Apply colors if enabled
	if l.useColors {
		colorCode := levelColors[level]
		colorReset := "\033[0m"
		prefix = fmt.Sprintf("%s%s%s", colorCode, prefix, colorReset)
	}

	// Log the message with the formatted prefix
	l.logger.Println(prefix, fmt.Sprint(v...))

	// If FATAL, exit the program
	if level == FATAL {
		if l.file != nil {
			l.file.Close()
		}
		os.Exit(1)
	}
}

// logf logs a formatted message with the specified level
func (l *Logger) logf(level LogLevel, format string, v ...interface{}) {
	if level < l.level {
		return
	}

	// Get caller info
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "unknown"
		line = 0
	}

	// Format the file path to just the file name
	file = filepath.Base(file)

	// Get the current time
	now := time.Now().Format("2006/01/02 15:04:05")

	// Format the log prefix
	prefix := fmt.Sprintf("%s [%s] %s:%d:", now, levelPrefixes[level], file, line)

	// Apply colors if enabled
	if l.useColors {
		colorCode := levelColors[level]
		colorReset := "\033[0m"
		prefix = fmt.Sprintf("%s%s%s", colorCode, prefix, colorReset)
	}

	// Log the formatted message with the formatted prefix
	l.logger.Println(prefix, fmt.Sprintf(format, v...))

	// If FATAL, exit the program
	if level == FATAL {
		if l.file != nil {
			l.file.Close()
		}
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(v ...interface{}) {
	l.log(DEBUG, v...)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, v ...interface{}) {
	l.logf(DEBUG, format, v...)
}

// Info logs an info message
func (l *Logger) Info(v ...interface{}) {
	l.log(INFO, v...)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, v ...interface{}) {
	l.logf(INFO, format, v...)
}

// Warn logs a warning message
func (l *Logger) Warn(v ...interface{}) {
	l.log(WARN, v...)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, v ...interface{}) {
	l.logf(WARN, format, v...)
}

// Error logs an error message
func (l *Logger) Error(v ...interface{}) {
	l.log(ERROR, v...)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, v ...interface{}) {
	l.logf(ERROR, format, v...)
}

// Fatal logs a fatal message and exits the program
func (l *Logger) Fatal(v ...interface{}) {
	l.log(FATAL, v...)
}

// Fatalf logs a formatted fatal message and exits the program
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.logf(FATAL, format, v...)
}

// SetLevel sets the log level
func (l *Logger) SetLevel(levelStr string) {
	// Parse log level
	switch strings.ToLower(levelStr) {
	case "debug":
		l.level = DEBUG
	case "info":
		l.level = INFO
	case "warn":
		l.level = WARN
	case "error":
		l.level = ERROR
	case "fatal":
		l.level = FATAL
	default:
		l.level = INFO // Default to INFO
	}
}

// SetOutput sets the output writer for the logger
func (l *Logger) SetOutput(w io.Writer) {
	l.logger.SetOutput(w)
}

// EnableColors enables or disables colored output
func (l *Logger) EnableColors(enable bool) {
	l.useColors = enable
}

// Close closes the logger's file if it exists
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}
