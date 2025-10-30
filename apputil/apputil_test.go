package apputil

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/science-computing/service-common-golang/apputil/slogverbosetext"
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

func TestVerboseLoggingSourceInfo(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer

	// Create our custom verbose text handler with INFO level
	opts := &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}

	// We need to import the slogverbosetext package
	handler := slogverbosetext.New(&buf, opts)
	slogger := slog.New(handler)
	testLogger := &LoggerWrapper{Logger: slogger}

	// Call Infof from this specific line - remember this line number!
	testLogger.Infof("Test info message: %s", "hello") // Line 116

	// Get the output
	output := buf.String()

	// Verify the message is present
	if !strings.Contains(output, "Test info message: hello") {
		t.Errorf("Expected log message not found in output: %s", output)
	}

	// Verify the source file is correct
	if !strings.Contains(output, "apputil_test.go") {
		t.Errorf("Expected source file 'apputil_test.go' in output, got: %s", output)
	}

	// Verify the line number is present (checking for the line where Infof was called)
	// Note: This line number should match the line where Infof is called above
	expectedLineRef := "apputil_test.go:116"
	if !strings.Contains(output, expectedLineRef) {
		t.Errorf("Expected source reference '%s' in output, got: %s", expectedLineRef, output)
	}

	// Verify INFO level is shown
	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected log level 'INFO' in output, got: %s", output)
	}
}

func TestVerboseLoggingFormat(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer

	// Create our custom verbose text handler with INFO level
	opts := &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}

	handler := slogverbosetext.New(&buf, opts)
	slogger := slog.New(handler)
	testLogger := &LoggerWrapper{Logger: slogger}

	// Log a message
	testLogger.Info("Test message")

	output := buf.String()

	// Verify format components are present
	// Format should be: [COLOR]LEVEL[/COLOR][TIMESTAMP] MESSAGE -- FILE:LINE

	// Check for level (INFO should be right-aligned to 6 chars based on handler)
	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected 'INFO' level in output: %s", output)
	}

	// Check for timestamp in format [YYYY-MM-DD HH:MM:SS]
	if !strings.Contains(output, "[2025-") {
		t.Errorf("Expected timestamp with format [2025-...] in output: %s", output)
	}

	// Check for message
	if !strings.Contains(output, "Test message") {
		t.Errorf("Expected 'Test message' in output: %s", output)
	}

	// Check for separator before file:line
	if !strings.Contains(output, " -- ") {
		t.Errorf("Expected ' -- ' separator in output: %s", output)
	}

	// Check for file:line format
	if !strings.Contains(output, "apputil_test.go:") {
		t.Errorf("Expected 'apputil_test.go:LINE' in output: %s", output)
	}
}
