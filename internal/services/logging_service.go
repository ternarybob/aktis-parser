// -----------------------------------------------------------------------
// Last Modified: Thursday, 2nd October 2025 9:46:53 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package services

import (
	"fmt"
	"sync"

	"aktis-parser/internal/interfaces"
	. "github.com/ternarybob/arbor"
)

// AppLoggingService is the concrete implementation of LoggingService
type AppLoggingService struct {
	logger   ILogger
	uiLogger UILogger
	mu       sync.RWMutex
	fields   map[string]interface{}
}

// UILogger interface for broadcasting to UI
type UILogger interface {
	BroadcastUILog(level, message string)
}

// NewLoggingService creates a new logging service instance
func NewLoggingService(logger ILogger) *AppLoggingService {
	return &AppLoggingService{
		logger: logger,
		fields: make(map[string]interface{}),
	}
}

// SetUILogger sets the UI logger for broadcasting to WebSocket clients
func (s *AppLoggingService) SetUILogger(uiLogger UILogger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uiLogger = uiLogger
}

// Debug logs a debug message
func (s *AppLoggingService) Debug(message string) {
	s.logger.Debug().Msg(message)
}

// Info logs an info message
func (s *AppLoggingService) Info(message string) {
	s.logger.Info().Msg(message)
	s.BroadcastToUI("info", message)
}

// Warn logs a warning message
func (s *AppLoggingService) Warn(message string) {
	s.logger.Warn().Msg(message)
	s.BroadcastToUI("warn", message)
}

// Error logs an error message
func (s *AppLoggingService) Error(message string, err error) {
	if err != nil {
		s.logger.Error().Err(err).Msg(message)
	} else {
		s.logger.Error().Msg(message)
	}
	s.BroadcastToUI("error", message)
}

// WithField returns a log entry with a single field
func (s *AppLoggingService) WithField(key string, value interface{}) interfaces.LogEntry {
	return &logEntryImpl{
		service: s,
		fields:  map[string]interface{}{key: value},
	}
}

// WithFields returns a log entry with multiple fields
func (s *AppLoggingService) WithFields(fields map[string]interface{}) interfaces.LogEntry {
	return &logEntryImpl{
		service: s,
		fields:  fields,
	}
}

// BroadcastToUI broadcasts a log message to UI clients
func (s *AppLoggingService) BroadcastToUI(level, message string) {
	s.mu.RLock()
	uiLogger := s.uiLogger
	s.mu.RUnlock()

	if uiLogger != nil {
		uiLogger.BroadcastUILog(level, message)
	}
}

// logEntryImpl implements the LogEntry interface
type logEntryImpl struct {
	service *AppLoggingService
	fields  map[string]interface{}
}

func (e *logEntryImpl) applyFields(event ILogEvent) ILogEvent {
	for key, value := range e.fields {
		switch v := value.(type) {
		case string:
			event = event.Str(key, v)
		case int:
			event = event.Int(key, v)
		case error:
			event = event.Err(v)
		default:
			// For other types, convert to string
			event = event.Str(key, fmt.Sprintf("%v", v))
		}
	}
	return event
}

func (e *logEntryImpl) Debug(message string) {
	event := e.service.logger.Debug()
	e.applyFields(event).Msg(message)
}

func (e *logEntryImpl) Info(message string) {
	event := e.service.logger.Info()
	e.applyFields(event).Msg(message)
	e.service.BroadcastToUI("info", message)
}

func (e *logEntryImpl) Warn(message string) {
	event := e.service.logger.Warn()
	e.applyFields(event).Msg(message)
	e.service.BroadcastToUI("warn", message)
}

func (e *logEntryImpl) Error(message string, err error) {
	event := e.service.logger.Error()
	if err != nil {
		event = event.Err(err)
	}
	e.applyFields(event).Msg(message)
	e.service.BroadcastToUI("error", message)
}

func (e *logEntryImpl) Msg(message string) {
	e.Info(message)
}
