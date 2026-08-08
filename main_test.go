package main

import (
	"testing"
	"time"
)

func TestClassifyLevel(t *testing.T) {
	tests := []struct {
		line string
		want LogLevel
	}{
		{"INFO Starting server", LevelInfo},
		{"ERROR connection refused", LevelError},
		{"WARN deprecated API", LevelWarn},
		{"FATAL out of memory", LevelFatal},
		{"[ERROR] something failed", LevelError},
		{"[WARNING] low memory", LevelWarn},
		{"normal log line", LevelInfo},
		{"CRITICAL disk full", LevelFatal},
	}
	for _, tt := range tests {
		got := classifyLevel(tt.line)
		if got != tt.want {
			t.Errorf("classifyLevel(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestExtractSource(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"[nginx] GET /api", "nginx"},
		{"[redis] connection", "redis"},
		{"2026-08-08 INFO normal line", "INFO"},
	}
	for _, tt := range tests {
		got := extractSource(tt.line)
		if got != tt.want {
			t.Errorf("extractSource(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestParseTimestamp(t *testing.T) {
	line := "2026-08-08T14:30:00Z something happened"
	ts := parseTimestamp(line)
	if ts.IsZero() {
		t.Error("Expected valid timestamp")
	}
	if ts.Year() != 2026 {
		t.Errorf("Expected year 2026, got %d", ts.Year())
	}
}

func TestParseTimestampEmpty(t *testing.T) {
	ts := parseTimestamp("no timestamp here")
	if !ts.IsZero() {
		t.Error("Expected zero time for line without timestamp")
	}
}

func TestAnalyzeLogsEmpty(t *testing.T) {
	result := analyzeLogs([]LogEntry{})
	if result.TotalLines != 0 {
		t.Errorf("Expected 0 lines, got %d", result.TotalLines)
	}
}

func TestAnalyzeLogsAllLevels(t *testing.T) {
	entries := []LogEntry{
		{Level: LevelInfo, Message: "info1"},
		{Level: LevelInfo, Message: "info2"},
		{Level: LevelWarn, Message: "warn1"},
		{Level: LevelError, Message: "error1"},
		{Level: LevelFatal, Message: "fatal1"},
	}
	result := analyzeLogs(entries)
	if result.ByLevel[LevelInfo] != 2 {
		t.Errorf("Expected 2 info, got %d", result.ByLevel[LevelInfo])
	}
	if result.ByLevel[LevelError] != 1 {
		t.Errorf("Expected 1 error, got %d", result.ByLevel[LevelError])
	}
}

func TestAnalyzeLogsTimeline(t *testing.T) {
	entries := []LogEntry{
		{Timestamp: time.Date(2026, 8, 8, 14, 30, 0, 0, time.UTC), Level: LevelInfo, Message: "ok"},
		{Timestamp: time.Date(2026, 8, 8, 14, 30, 0, 0, time.UTC), Level: LevelError, Message: "fail"},
		{Timestamp: time.Date(2026, 8, 8, 14, 31, 0, 0, time.UTC), Level: LevelWarn, Message: "warn"},
	}
	result := analyzeLogs(entries)
	if len(result.Timeline) != 2 {
		t.Errorf("Expected 2 timeline buckets, got %d", len(result.Timeline))
	}
}

func TestAnalyzeLogsTopErrors(t *testing.T) {
	entries := []LogEntry{
		{Level: LevelError, Message: "connection refused"},
		{Level: LevelError, Message: "connection refused"},
		{Level: LevelError, Message: "connection refused"},
		{Level: LevelError, Message: "timeout"},
	}
	result := analyzeLogs(entries)
	if len(result.TopErrors) == 0 {
		t.Error("Expected top errors")
	}
	if result.TopErrors[0].Count != 3 {
		t.Errorf("Expected count 3, got %d", result.TopErrors[0].Count)
	}
}

func TestAnalyzeLogsRecommendations(t *testing.T) {
	entries := []LogEntry{
		{Level: LevelFatal, Message: "crash"},
	}
	result := analyzeLogs(entries)
	if len(result.Recommendations) == 0 {
		t.Error("Expected recommendations for fatal errors")
	}
}