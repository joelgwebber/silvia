package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// Logger provides logging capabilities for tool execution
type Logger interface {
	LogToolCall(toolName string, args map[string]interface{})
	LogToolResult(toolName string, result ToolResult, duration time.Duration)
	LogToolError(toolName string, err error)
	LogChainStart(chainLength int)
	LogChainComplete(duration time.Duration, success bool)
}

// DefaultLogger implements Logger with stdout/stderr logging
type DefaultLogger struct {
	verbose bool
	prefix  string
	logger  *log.Logger
}

// NewDefaultLogger creates a new default logger
func NewDefaultLogger(verbose bool) *DefaultLogger {
	return &DefaultLogger{
		verbose: verbose,
		prefix:  "[TOOLS]",
		logger:  log.New(os.Stdout, "", log.LstdFlags),
	}
}

// LogToolCall logs when a tool is called
func (l *DefaultLogger) LogToolCall(toolName string, args map[string]interface{}) {
	argsJSON, _ := json.Marshal(args)
	l.logger.Printf("%s CALL: %s with args: %s", l.prefix, toolName, string(argsJSON))
}

// LogToolResult logs the result of a tool execution
func (l *DefaultLogger) LogToolResult(toolName string, result ToolResult, duration time.Duration) {
	status := "SUCCESS"
	if !result.Success {
		status = "FAILED"
	}

	if l.verbose {
		// In verbose mode, log the full result
		dataJSON, _ := json.MarshalIndent(result.Data, "", "  ")
		l.logger.Printf("%s RESULT: %s [%s] (%.3fs)\n  Data: %s\n  Meta: %v",
			l.prefix, toolName, status, duration.Seconds(), string(dataJSON), result.Meta)
	} else {
		// In normal mode, just log status and metadata
		l.logger.Printf("%s RESULT: %s [%s] (%.3fs) Meta: %v",
			l.prefix, toolName, status, duration.Seconds(), result.Meta)
	}

	if result.Error != "" {
		l.logger.Printf("%s ERROR: %s - %s", l.prefix, toolName, result.Error)
	}
}

// LogToolError logs when a tool encounters an error
func (l *DefaultLogger) LogToolError(toolName string, err error) {
	l.logger.Printf("%s ERROR: %s - %v", l.prefix, toolName, err)
}

// LogChainStart logs the start of a tool chain execution
func (l *DefaultLogger) LogChainStart(chainLength int) {
	l.logger.Printf("%s CHAIN START: Executing %d tools", l.prefix, chainLength)
}

// LogChainComplete logs the completion of a tool chain
func (l *DefaultLogger) LogChainComplete(duration time.Duration, success bool) {
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}
	l.logger.Printf("%s CHAIN COMPLETE: [%s] (%.3fs)", l.prefix, status, duration.Seconds())
}

// NullLogger implements Logger but does nothing (for when logging is disabled)
type NullLogger struct{}

// LogToolCall does nothing
func (n *NullLogger) LogToolCall(toolName string, args map[string]interface{}) {}

// LogToolResult does nothing
func (n *NullLogger) LogToolResult(toolName string, result ToolResult, duration time.Duration) {}

// LogToolError does nothing
func (n *NullLogger) LogToolError(toolName string, err error) {}

// LogChainStart does nothing
func (n *NullLogger) LogChainStart(chainLength int) {}

// LogChainComplete does nothing
func (n *NullLogger) LogChainComplete(duration time.Duration, success bool) {}

// FileLogger logs to a file
type FileLogger struct {
	*DefaultLogger
	file *os.File
}

// NewFileLogger creates a logger that writes to a file
func NewFileLogger(filename string, verbose bool) (*FileLogger, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &FileLogger{
		DefaultLogger: &DefaultLogger{
			verbose: verbose,
			prefix:  "[TOOLS]",
			logger:  log.New(file, "", log.LstdFlags),
		},
		file: file,
	}, nil
}

// Close closes the log file
func (f *FileLogger) Close() error {
	if f.file != nil {
		return f.file.Close()
	}
	return nil
}
