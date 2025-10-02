package common

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/arbor/models"
)

var (
	logger arbor.ILogger
	mu     sync.RWMutex
)

func GetLogger() arbor.ILogger {
	mu.RLock()
	if logger != nil {
		mu.RUnlock()
		return logger
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	// Double-check after acquiring write lock
	if logger == nil {
		logger = _initDefaultLogger()
	}
	return logger
}

// GetLogFilePath returns the actual configured log file path from the arbor logger
func GetLogFilePath() string {
	mu.RLock()
	currentLogger := logger
	mu.RUnlock()

	if currentLogger != nil {
		if logFilePath := currentLogger.GetLogFilePath(); logFilePath != "" {
			return logFilePath
		}
	}

	// Fallback to default path if logger not initialized or no file writer configured
	execPath, err := os.Executable()
	if err != nil {
		return "logs/aktis-parser.log" // Return relative path as fallback
	}
	execDir := filepath.Dir(execPath)
	return filepath.Join(execDir, "logs", "aktis-parser.log")
}

func InitLogger(config *LoggingConfig) error {
	mu.Lock()
	defer mu.Unlock()

	if logger != nil {
		return nil // Already initialized
	}

	var err error
	logger, err = _createLogger(config)
	return err
}

func _initDefaultLogger() arbor.ILogger {
	config := _defaultLoggingConfig()
	logger, err := _createLogger(config)
	if err != nil {
		fmt.Printf("Warning: Failed to initialize default logger: %v\n", err)
		return arbor.NewLogger()
	}
	return logger
}

func _createLogger(config *LoggingConfig) (arbor.ILogger, error) {
	// Get the directory where the executable is located
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)

	// Create logs directory in the same directory as the executable
	logsDir := filepath.Join(execDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Initialize arbor logger
	l := arbor.NewLogger()

	// Configure file logging if requested
	if config.Output == "both" || config.Output == "file" || config.Output == "" {
		logFile := filepath.Join(logsDir, "aktis-parser.log")
		l = l.WithFileWriter(models.WriterConfiguration{
			Type:             models.LogWriterTypeFile,
			FileName:         logFile,
			TimeFormat:       "15:04:05",
			MaxSize:          int64(config.MaxSize * 1024 * 1024), // Convert MB to bytes
			MaxBackups:       config.MaxBackups,
			TextOutput:       true,
			DisableTimestamp: false,
		})
	}

	// Configure console logging if requested
	if config.Output == "both" || config.Output == "console" || config.Output == "" {
		l = l.WithConsoleWriter(models.WriterConfiguration{
			Type:             models.LogWriterTypeConsole,
			TimeFormat:       "15:04:05",
			TextOutput:       true,
			DisableTimestamp: false,
		})
	}

	// Always add memory writer for WebSocket log streaming (respects config but memory is always on)
	l = l.WithMemoryWriter(models.WriterConfiguration{
		Type:             models.LogWriterTypeMemory,
		TimeFormat:       "15:04:05",
		TextOutput:       true,
		DisableTimestamp: false,
	})

	// Set log level
	l = l.WithLevelFromString(config.Level)

	return l, nil
}

func _defaultLoggingConfig() *LoggingConfig {
	return &LoggingConfig{
		Level:      "info",
		Format:     "text",
		Output:     "both",
		MaxSize:    100,
		MaxBackups: 3,
	}
}
