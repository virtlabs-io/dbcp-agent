package logger

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestSetLevelParsing(t *testing.T) {
	cases := []struct {
		input    string
		expected LogLevel
	}{
		{"error", ErrorLevel},
		{"warn", WarnLevel},
		{"info", InfoLevel},
		{"debug", DebugLevel},
		{"unknown", InfoLevel}, // fallback
	}

	for _, c := range cases {
		SetLevel(c.input)
		if currentLevel != c.expected {
			t.Errorf("SetLevel(%q) = %v; want %v", c.input, currentLevel, c.expected)
		}
	}
}

func TestLogLevelFiltering(t *testing.T) {
	var buf bytes.Buffer

	Init(Options{
		Level:  "warn",
		Output: "stdout",
	})

	// Override writers for test
	errorLogger = newTestLogger(&buf, "[ERROR] ")
	warnLogger = newTestLogger(&buf, "[WARN]  ")
	infoLogger = newTestLogger(&buf, "[INFO]  ")
	debugLogger = newTestLogger(&buf, "[DEBUG] ")

	Error("this is an error: %d", 123)
	Warn("this is a warning")
	Info("this info should not appear")
	Debug("nor this debug")

	output := buf.String()

	if !strings.Contains(output, "this is an error") {
		t.Error("expected error log to be present")
	}
	if !strings.Contains(output, "this is a warning") {
		t.Error("expected warning log to be present")
	}
	if strings.Contains(output, "this info should not appear") {
		t.Error("did not expect info log at warn level")
	}
	if strings.Contains(output, "nor this debug") {
		t.Error("did not expect debug log at warn level")
	}
}

func newTestLogger(w *bytes.Buffer, prefix string) *log.Logger {
	return log.New(w, prefix, 0)
}
