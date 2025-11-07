package util

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// LoggerConfig holds configuration for logger initialization
type LoggerConfig struct {
	// LogFilePath is the path to the log file. If empty, logs to stdout only.
	LogFilePath string
	// LogLevel is the minimum log level to output
	LogLevel slog.Level
}

// NewLogger creates a new structured logger that writes JSON logs to a file and stdout
func NewLogger(config LoggerConfig) (*slog.Logger, io.Closer, error) {
	var writer io.Writer
	var closer io.Closer

	if config.LogFilePath != "" {
		// Create logs directory if it doesn't exist
		logDir := filepath.Dir(config.LogFilePath)
		if logDir != "" && logDir != "." {
			if err := os.MkdirAll(logDir, 0755); err != nil {
				return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
			}
		}

		// Open log file for writing (create if not exists, append if exists)
		file, err := os.OpenFile(config.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open log file: %w", err)
		}

		// Use both stdout and file for logging
		writer = io.MultiWriter(os.Stdout, file)
		closer = file
	} else {
		// Default to stdout only
		writer = os.Stdout
		closer = nil
	}

	// Create JSON handler with specified log level
	opts := &slog.HandlerOptions{
		Level: config.LogLevel,
	}
	handler := slog.NewJSONHandler(writer, opts)
	logger := slog.New(handler)

	return logger, closer, nil
}

// GenerateLogFileName generates a log file name with timestamp in the logs directory
func GenerateLogFileName(commandName string) string {
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join("logs", fmt.Sprintf("%s-%s.json", commandName, timestamp))
}
