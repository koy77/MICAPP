// MIT License
// Copyright (c) 2024 VoiceTranscriber
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

package main

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

// LogLevel represents the logging level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String returns the string representation of LogLevel
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

// AppLogger represents the application logger
type AppLogger struct {
	level    LogLevel
	logger   *log.Logger
	file     *os.File
	filePath string
}

// NewAppLogger creates a new application logger
func NewAppLogger(level LogLevel) (*AppLogger, error) {
	// Use single app.log file in root directory
	logFileName := "app.log"
	filePath := logFileName

	// Open log file
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	// Create multi-writer to write to both file and console
	multiWriter := io.MultiWriter(file, os.Stdout)

	// Create logger with custom format
	logger := log.New(multiWriter, "", 0)

	appLogger := &AppLogger{
		level:    level,
		logger:   logger,
		file:     file,
		filePath: filePath,
	}

	// Log initial message
	appLogger.Info("Logger initialized", "level", level.String(), "file", filePath)

	return appLogger, nil
}

// Close closes the log file
func (l *AppLogger) Close() error {
	if l.file != nil {
		l.Info("Logger closing")
		return l.file.Close()
	}
	return nil
}

// SetLevel sets the logging level
func (l *AppLogger) SetLevel(level LogLevel) {
	l.level = level
	l.Info("Log level changed", "new_level", level.String())
}

// GetLevel returns the current logging level
func (l *AppLogger) GetLevel() LogLevel {
	return l.level
}

// GetFilePath returns the log file path
func (l *AppLogger) GetFilePath() string {
	return l.filePath
}

// log writes a log message with the specified level
func (l *AppLogger) log(level LogLevel, message string, fields ...interface{}) {
	if level < l.level {
		return
	}

	// Get caller information
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "unknown"
		line = 0
	}

	// Extract filename from full path
	fileName := filepath.Base(file)

	// Format timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	// Format fields
	var fieldStr string
	if len(fields) > 0 {
		var parts []string
		for i := 0; i < len(fields); i += 2 {
			if i+1 < len(fields) {
				parts = append(parts, fmt.Sprintf("%v=%v", fields[i], fields[i+1]))
			}
		}
		if len(parts) > 0 {
			fieldStr = " " + strings.Join(parts, " ")
		}
	}

	// Create log message
	logMessage := fmt.Sprintf("[%s] [%s] [%s:%d] %s%s",
		timestamp,
		level.String(),
		fileName,
		line,
		message,
		fieldStr,
	)

	// Write log message
	l.logger.Println(logMessage)

	// For FATAL level, also exit the program
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *AppLogger) Debug(message string, fields ...interface{}) {
	l.log(DEBUG, message, fields...)
}

// Info logs an info message
func (l *AppLogger) Info(message string, fields ...interface{}) {
	l.log(INFO, message, fields...)
}

// Warn logs a warning message
func (l *AppLogger) Warn(message string, fields ...interface{}) {
	l.log(WARN, message, fields...)
}

// Error logs an error message
func (l *AppLogger) Error(message string, fields ...interface{}) {
	l.log(ERROR, message, fields...)
}

// Fatal logs a fatal message and exits the program
func (l *AppLogger) Fatal(message string, fields ...interface{}) {
	l.log(FATAL, message, fields...)
}

// Debugf logs a formatted debug message
func (l *AppLogger) Debugf(format string, args ...interface{}) {
	if DEBUG >= l.level {
		l.Debug(fmt.Sprintf(format, args...))
	}
}

// Infof logs a formatted info message
func (l *AppLogger) Infof(format string, args ...interface{}) {
	if INFO >= l.level {
		l.Info(fmt.Sprintf(format, args...))
	}
}

// Warnf logs a formatted warning message
func (l *AppLogger) Warnf(format string, args ...interface{}) {
	if WARN >= l.level {
		l.Warn(fmt.Sprintf(format, args...))
	}
}

// Errorf logs a formatted error message
func (l *AppLogger) Errorf(format string, args ...interface{}) {
	if ERROR >= l.level {
		l.Error(fmt.Sprintf(format, args...))
	}
}

// Fatalf logs a formatted fatal message and exits the program
func (l *AppLogger) Fatalf(format string, args ...interface{}) {
	if FATAL >= l.level {
		l.Fatal(fmt.Sprintf(format, args...))
	}
}

// LogAudioEvent logs audio-related events
func (l *AppLogger) LogAudioEvent(event string, duration time.Duration, sampleRate int, channels int) {
	l.Info("Audio event",
		"event", event,
		"duration", duration.String(),
		"sample_rate", sampleRate,
		"channels", channels,
	)
}

// LogTranscriptionEvent logs transcription-related events
func (l *AppLogger) LogTranscriptionEvent(event string, language string, textLength int, processingTime time.Duration) {
	l.Info("Transcription event",
		"event", event,
		"language", language,
		"text_length", textLength,
		"processing_time", processingTime.String(),
	)
}

// LogLLMEvent logs LLM-related events
func (l *AppLogger) LogLLMEvent(event string, model string, tokens int, processingTime time.Duration) {
	l.Info("LLM event",
		"event", event,
		"model", model,
		"tokens", tokens,
		"processing_time", processingTime.String(),
	)
}

// LogUIEvent logs UI-related events
func (l *AppLogger) LogUIEvent(event string, component string, action string) {
	l.Debug("UI event",
		"event", event,
		"component", component,
		"action", action,
	)
}

// LogError logs an error with context
func (l *AppLogger) LogError(err error, context string, fields ...interface{}) {
	errorFields := append([]interface{}{"error", err.Error(), "context", context}, fields...)
	l.Error("Application error", errorFields...)
}

// LogPerformance logs performance metrics
func (l *AppLogger) LogPerformance(operation string, duration time.Duration, memoryUsage int64) {
	l.Info("Performance metric",
		"operation", operation,
		"duration", duration.String(),
		"memory_usage", memoryUsage,
	)
}

// Global logger instance
var globalLogger *AppLogger

// InitLogger initializes the global logger
func InitLogger(level LogLevel) error {
	var err error
	globalLogger, err = NewAppLogger(level)
	return err
}

// GetLogger returns the global logger instance
func GetLogger() *AppLogger {
	if globalLogger == nil {
		// Initialize with INFO level if not already initialized
		InitLogger(INFO)
	}
	return globalLogger
}

// Convenience functions for global logger
func Debug(message string, fields ...interface{}) {
	GetLogger().Debug(message, fields...)
}

func Info(message string, fields ...interface{}) {
	GetLogger().Info(message, fields...)
}

func Warn(message string, fields ...interface{}) {
	GetLogger().Warn(message, fields...)
}

func Error(message string, fields ...interface{}) {
	GetLogger().Error(message, fields...)
}

func Fatal(message string, fields ...interface{}) {
	GetLogger().Fatal(message, fields...)
}

func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}
