package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseLogLineDetectsLevels(t *testing.T) {
	cases := map[string]string{
		"2026-08-01 10:00:00 INFO starting up":       "INFO",
		"2026-08-01 10:00:01 ERROR connection reset": "ERROR",
		"2026-08-01 10:00:02 FATAL out of memory":    "ERROR",
		"2026-08-01 10:00:03 PANIC nil pointer":      "ERROR",
		"2026-08-01 10:00:04 WARN slow query":        "WARN",
		"2026-08-01 10:00:05 DEBUG cache hit":        "DEBUG",
		"plain line with no level":                   "INFO",
	}
	for line, want := range cases {
		if got := parseLogLine(line).Level; got != want {
			t.Errorf("parseLogLine(%q).Level = %q, want %q", line, got, want)
		}
	}
}

func TestParseLogLineReadsTimestamp(t *testing.T) {
	entry := parseLogLine("2026-08-01 10:00:00 ERROR boom")
	if entry.Timestamp.IsZero() {
		t.Fatal("timestamp was not parsed")
	}
	if entry.Timestamp.Format("2006-01-02 15:04:05") != "2026-08-01 10:00:00" {
		t.Errorf("unexpected timestamp: %v", entry.Timestamp)
	}
}

func TestParseLogLineKeepsTheMessage(t *testing.T) {
	line := "2026-08-01T10:00:00Z ERROR something broke"
	if got := parseLogLine(line).Message; got != line {
		t.Errorf("message = %q, want the raw line", got)
	}
}

func TestAnalyzeLogsCountsEveryLevel(t *testing.T) {
	entries := []LogEntry{
		{Level: "ERROR"}, {Level: "ERROR"}, {Level: "WARN"},
		{Level: "INFO"}, {Level: "DEBUG"}, {Level: "INFO"},
	}
	result := analyzeLogs(entries)
	if result.TotalLines != 6 || result.ErrorCount != 2 || result.WarnCount != 1 ||
		result.InfoCount != 2 || result.DebugCount != 1 {
		t.Errorf("unexpected counts: %+v", result)
	}
	if result.LevelCounts["ERROR"] != 2 {
		t.Errorf("level counts not filled: %+v", result.LevelCounts)
	}
}

func TestAnalyzeLogsRanksTopErrorsByFrequency(t *testing.T) {
	entries := []LogEntry{
		{Level: "ERROR", Message: "common"},
		{Level: "ERROR", Message: "rare"},
		{Level: "ERROR", Message: "common"},
		{Level: "ERROR", Message: "common"},
	}
	result := analyzeLogs(entries)
	if len(result.TopErrors) != 2 {
		t.Fatalf("expected 2 distinct errors, got %d", len(result.TopErrors))
	}
	if result.TopErrors[0].Message != "common" || result.TopErrors[0].Count != 3 {
		t.Errorf("top error = %+v", result.TopErrors[0])
	}
}

func TestAnalyzeLogsIgnoresTimestampsWhenGroupingErrors(t *testing.T) {
	entries := []LogEntry{
		{Level: "ERROR", Message: "2026-08-01 09:59:01 ERROR connection reset by peer"},
		{Level: "ERROR", Message: "2026-08-01 09:59:02 ERROR connection reset by peer"},
		{Level: "ERROR", Message: "2026-08-01T09:59:05Z ERROR connection reset by peer"},
	}
	result := analyzeLogs(entries)
	if len(result.TopErrors) != 1 {
		t.Fatalf("expected one grouped error, got %+v", result.TopErrors)
	}
	if result.TopErrors[0].Count != 3 || result.TopErrors[0].Message != "connection reset by peer" {
		t.Errorf("grouping failed: %+v", result.TopErrors[0])
	}
}

func TestNormalizeErrorMessageKeepsBareLines(t *testing.T) {
	if got := normalizeErrorMessage("boom"); got != "boom" {
		t.Errorf("normalizeErrorMessage(boom) = %q", got)
	}
	if got := normalizeErrorMessage("2026-08-01 10:00:00 ERROR"); got == "" {
		t.Error("a line with only a timestamp and level should not become empty")
	}
}

func TestNormalizeErrorMessageStripsRevisionAndLevels(t *testing.T) {
	cases := map[string]string{
		"2026-08-01 10:00:00 WARN slow query":          "slow query",
		"2026-08-01T10:00:00.123Z FATAL out of memory": "out of memory",
		"ERROR: disk full":                             "disk full",
		"INFO  ready":                                  "ready",
	}
	for input, want := range cases {
		if got := normalizeErrorMessage(input); got != want {
			t.Errorf("normalizeErrorMessage(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAnalyzeLogsTruncatesLongErrorMessages(t *testing.T) {
	long := strings.Repeat("x", 500)
	result := analyzeLogs([]LogEntry{{Level: "ERROR", Message: long}})
	if len(result.TopErrors[0].Message) != 120 {
		t.Errorf("expected 120 characters, got %d", len(result.TopErrors[0].Message))
	}
}

func TestAnalyzeLogsBuildsTimelineAndPodStats(t *testing.T) {
	ts := time.Date(2026, 8, 1, 10, 15, 0, 0, time.UTC)
	entries := []LogEntry{
		{Level: "INFO", Timestamp: ts, Pod: "web-1", Namespace: "prod"},
		{Level: "INFO", Timestamp: ts.Add(time.Minute), Pod: "web-1", Namespace: "prod"},
		{Level: "INFO", Timestamp: ts, Pod: "db-0", Namespace: "prod"},
	}
	result := analyzeLogs(entries)
	if result.Timeline["10:15"] != 2 || result.Timeline["10:16"] != 1 {
		t.Errorf("timeline = %+v", result.Timeline)
	}
	if result.PodStats["web-1"] != 2 || result.NamespaceStats["prod"] != 3 {
		t.Errorf("pod stats = %+v, namespaces = %+v", result.PodStats, result.NamespaceStats)
	}
}

func TestAnalyzeLogsOnEmptyInput(t *testing.T) {
	result := analyzeLogs(nil)
	if result.TotalLines != 0 || len(result.TopErrors) != 0 {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestWriteJSONProducesADecodableReport(t *testing.T) {
	result := analyzeLogs([]LogEntry{{Level: "ERROR", Message: "boom"}})
	var buf bytes.Buffer
	if err := writeJSON(&buf, result); err != nil {
		t.Fatal(err)
	}

	var report Report
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if report.Summary.ErrorCount != 1 {
		t.Errorf("summary lost the error count: %+v", report.Summary)
	}
	if report.Timestamp == "" {
		t.Error("report has no timestamp")
	}
}

func TestWriteCSVIsDeterministicAndParseable(t *testing.T) {
	result := analyzeLogs([]LogEntry{
		{Level: "ERROR", Message: "boom", Timestamp: time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)},
		{Level: "INFO", Message: "ok", Timestamp: time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)},
	})

	first := renderCSV(t, result)
	second := renderCSV(t, result)
	if first != second {
		t.Errorf("CSV output changes between runs")
	}

	lines := strings.Split(strings.TrimSpace(first), "\n")
	if lines[0] != "metric,value" {
		t.Errorf("unexpected header: %q", lines[0])
	}
	if !strings.Contains(first, "error_count,1") {
		t.Errorf("missing error_count row:\n%s", first)
	}
	// Timeline must be chronological, not map order.
	if strings.Index(first, "09:00") > strings.Index(first, "10:00") {
		t.Errorf("timeline is not sorted:\n%s", first)
	}
}

func renderCSV(t *testing.T, result AnalysisResult) string {
	t.Helper()
	var buf bytes.Buffer
	if err := writeCSV(&buf, result); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if _, err := csv.NewReader(strings.NewReader(output)).ReadAll(); err != nil {
		t.Fatalf("CSV is not parseable: %v\n%s", err, output)
	}
	return output
}

func TestMinCapsBars(t *testing.T) {
	if min(3, 50) != 3 || min(80, 50) != 50 {
		t.Error("min does not cap correctly")
	}
}
