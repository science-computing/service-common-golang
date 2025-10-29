package apputil

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestLoggerWrapper(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	
	// Create a text handler that writes to our buffer
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewTextHandler(&buf, opts)
	slogger := slog.New(handler)
	
	// Create our wrapper
	logger := &LoggerWrapper{Logger: slogger}
	
	// Test different log levels
	logger.Debugf("Debug message: %s", "test")
	logger.Infof("Info message: %d", 42)
	logger.Warnf("Warning message")
	logger.Errorf("Error message with value: %v", true)
	
	// Check that output contains expected messages
	output := buf.String()
	
	if !strings.Contains(output, "Debug message: test") {
		t.Errorf("Expected debug message in output, got: %s", output)
	}
	
	if !strings.Contains(output, "Info message: 42") {
		t.Errorf("Expected info message in output, got: %s", output)
	}
	
	if !strings.Contains(output, "Warning message") {
		t.Errorf("Expected warning message in output, got: %s", output)
	}
	
	if !strings.Contains(output, "Error message with value: true") {
		t.Errorf("Expected error message in output, got: %s", output)
	}
}

func TestInitLogging(t *testing.T) {
	// Save original env vars
	originalLogfile := os.Getenv("CAEF_TEST_LOGFILE")
	defer func() {
		if originalLogfile != "" {
			os.Setenv("CAEF_TEST_LOGFILE", originalLogfile)
		} else {
			os.Unsetenv("CAEF_TEST_LOGFILE")
		}
	}()
	
	// Set up environment for testing
	upperProjectName = "CAEF"
	upperServiceName = "TEST"
	
	// Reset logger for testing
	logger = nil
	
	// Test InitLogging
	testLogger := InitLogging()
	if testLogger == nil {
		t.Error("InitLogging should not return nil")
	}
	
	// Test that calling InitLogging again returns the same logger
	testLogger2 := InitLogging()
	if testLogger != testLogger2 {
		t.Error("InitLogging should return the same logger instance")
	}
}

func TestInitLoggingWithLevel(t *testing.T) {
	// Reset logger for testing
	logger = nil
	
	// Test InitLoggingWithLevel with Debug level
	testLogger := InitLoggingWithLevel(slog.LevelDebug)
	if testLogger == nil {
		t.Error("InitLoggingWithLevel should not return nil")
	}
	
	// Verify it's a LoggerWrapper
	if _, ok := interface{}(testLogger).(*LoggerWrapper); !ok {
		t.Error("InitLoggingWithLevel should return a LoggerWrapper")
	}
}