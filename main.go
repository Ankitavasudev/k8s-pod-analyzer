package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Namespace string    `json:"namespace,omitempty"`
	Pod       string    `json:"pod,omitempty"`
	Container string    `json:"container,omitempty"`
}

type AnalysisResult struct {
	TotalLines     int            `json:"total_lines"`
	ErrorCount     int            `json:"error_count"`
	WarnCount      int            `json:"warn_count"`
	InfoCount      int            `json:"info_count"`
	DebugCount     int            `json:"debug_count"`
	TopErrors      []ErrorCount   `json:"top_errors"`
	Timeline       map[string]int `json:"timeline"`
	LevelCounts    map[string]int `json:"level_counts"`
	PodStats       map[string]int `json:"pod_stats,omitempty"`
	NamespaceStats map[string]int `json:"namespace_stats,omitempty"`
}

type ErrorCount struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

type Report struct {
	Timestamp string         `json:"timestamp"`
	Summary   AnalysisResult `json:"summary"`
	Entries   []LogEntry     `json:"entries,omitempty"`
}

func parseLogLine(line string) LogEntry {
	entry := LogEntry{Message: line}
	re := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2})`)
	if match := re.FindString(line); match != "" {
		// The regexp accepts a space as the separator, so try both layouts;
		// time.Parse with the T layout always failed on "2026-08-01 10:00:00".
		for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, match); err == nil {
				entry.Timestamp = t
				break
			}
		}
	}
	upper := strings.ToUpper(line)
	switch {
	case strings.Contains(upper, "ERROR") || strings.Contains(upper, "FATAL") || strings.Contains(upper, "PANIC"):
		entry.Level = "ERROR"
	case strings.Contains(upper, "WARN"):
		entry.Level = "WARN"
	case strings.Contains(upper, "DEBUG"):
		entry.Level = "DEBUG"
	default:
		entry.Level = "INFO"
	}
	return entry
}

var (
	timestampPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?\s*`)
	levelPattern     = regexp.MustCompile(`^(?i)(ERROR|FATAL|PANIC|WARN(?:ING)?|INFO|DEBUG)\s*[:|-]?\s*`)
)

// normalizeErrorMessage removes the timestamp and level prefix so the same
// failure from different seconds is counted once. Without this every line is
// unique and "top errors" is just "the last ten lines".
func normalizeErrorMessage(line string) string {
	trimmed := timestampPattern.ReplaceAllString(line, "")
	trimmed = levelPattern.ReplaceAllString(trimmed, "")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return strings.TrimSpace(line)
	}
	return trimmed
}

func analyzeLogs(entries []LogEntry) AnalysisResult {
	result := AnalysisResult{
		TotalLines:     len(entries),
		Timeline:       make(map[string]int),
		LevelCounts:    make(map[string]int),
		PodStats:       make(map[string]int),
		NamespaceStats: make(map[string]int),
	}
	errorMsgs := make(map[string]int)
	for _, e := range entries {
		result.LevelCounts[e.Level]++
		switch e.Level {
		case "ERROR":
			result.ErrorCount++
			msg := normalizeErrorMessage(e.Message)
			if len(msg) > 120 {
				msg = msg[:120]
			}
			errorMsgs[msg]++
		case "WARN":
			result.WarnCount++
		case "DEBUG":
			result.DebugCount++
		default:
			result.InfoCount++
		}
		if !e.Timestamp.IsZero() {
			result.Timeline[e.Timestamp.Format("15:04")]++
		}
		if e.Pod != "" {
			result.PodStats[e.Pod]++
		}
		if e.Namespace != "" {
			result.NamespaceStats[e.Namespace]++
		}
	}
	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range errorMsgs {
		sorted = append(sorted, kv{k, v})
	}
	// Break ties on the message so that two runs over the same log produce the
	// same order. Without it the top-ten list follows Go's map iteration order
	// and changes from run to run.
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Value != sorted[j].Value {
			return sorted[i].Value > sorted[j].Value
		}
		return sorted[i].Key < sorted[j].Key
	})
	for i, s := range sorted {
		if i >= 10 {
			break
		}
		result.TopErrors = append(result.TopErrors, ErrorCount{Message: s.Key, Count: s.Value})
	}
	return result
}

func getKubectlLogs(namespace, pod, container string, since int) ([]string, error) {
	args := []string{"logs"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, pod)
	if container != "" {
		args = append(args, "-c", container)
	}
	if since > 0 {
		args = append(args, fmt.Sprintf("--since=%dm", since))
	}
	args = append(args, "--tail=1000")
	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl logs failed: %v - %s", err, string(output))
	}
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, nil
}

func getPodList(namespace string) ([]string, error) {
	args := []string{"get", "pods", "-o", "jsonpath={.items[*].metadata.name}"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl get pods failed: %v - %s", err, string(output))
	}
	pods := strings.Fields(string(output))
	return pods, nil
}

func writeJSON(w io.Writer, result AnalysisResult) error {
	report := Report{
		Timestamp: time.Now().Format(time.RFC3339),
		Summary:   result,
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// writeCSV emits three small tables back to back: counts, the timeline, and the
// most frequent error messages. Timeline keys are sorted because map iteration
// order would otherwise make the output differ between runs.
func writeCSV(w io.Writer, result AnalysisResult) error {
	writer := csv.NewWriter(w)
	rows := [][]string{
		{"metric", "value"},
		{"total_lines", strconv.Itoa(result.TotalLines)},
		{"error_count", strconv.Itoa(result.ErrorCount)},
		{"warn_count", strconv.Itoa(result.WarnCount)},
		{"info_count", strconv.Itoa(result.InfoCount)},
		{"debug_count", strconv.Itoa(result.DebugCount)},
		{},
		{"timestamp", "count"},
	}

	timeline := make([]string, 0, len(result.Timeline))
	for timestamp := range result.Timeline {
		timeline = append(timeline, timestamp)
	}
	sort.Strings(timeline)
	for _, timestamp := range timeline {
		rows = append(rows, []string{timestamp, strconv.Itoa(result.Timeline[timestamp])})
	}

	rows = append(rows, []string{}, []string{"error_message", "count"})
	for _, entry := range result.TopErrors {
		rows = append(rows, []string{entry.Message, strconv.Itoa(entry.Count)})
	}

	if err := writer.WriteAll(rows); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

// version is the value used when the binary is built from a checkout. When the
// binary is installed from a module version, versionString reports that
// version instead, so the number in the report matches what was downloaded.
var version = "v0.2.0"

func versionString() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

// sortedKeys returns the map keys in ascending order so output does not depend
// on Go's randomised map iteration.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func filterByLevel(entries []LogEntry, level string) []LogEntry {
	if level == "" {
		return entries
	}
	level = strings.ToUpper(level)
	filtered := make([]LogEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Level == level {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// writeText renders the human-readable summary. The per-pod table is only
// printed for kubectl mode, where the logs of several pods are merged.
func writeText(w io.Writer, result AnalysisResult, timeline, showPods bool) error {
	fmt.Fprintf(w, "=== K8s Pod Log Analysis ===\n")
	fmt.Fprintf(w, "Total: %d | Errors: %d | Warnings: %d | Info: %d | Debug: %d\n",
		result.TotalLines, result.ErrorCount, result.WarnCount, result.InfoCount, result.DebugCount)
	if showPods && len(result.PodStats) > 0 {
		fmt.Fprintln(w, "\nPod Breakdown:")
		for _, pod := range sortedKeys(result.PodStats) {
			fmt.Fprintf(w, "  %s: %d lines\n", pod, result.PodStats[pod])
		}
	}
	if len(result.TopErrors) > 0 {
		fmt.Fprintln(w, "\nTop Errors:")
		for _, entry := range result.TopErrors {
			fmt.Fprintf(w, "  [%d times] %s\n", entry.Count, entry.Message)
		}
	}
	if timeline {
		fmt.Fprintln(w, "\nTimeline:")
		for _, minute := range sortedKeys(result.Timeline) {
			count := result.Timeline[minute]
			fmt.Fprintf(w, "  %s | %s %d\n", minute, strings.Repeat("#", min(count, 50)), count)
		}
	}
	return nil
}

type timelineRow struct {
	Minute  string
	Count   int
	Percent int
}

type namedCount struct {
	Name  string
	Count int
}

type htmlReport struct {
	Generated string
	Version   string
	Result    AnalysisResult
	Timeline  []timelineRow
	Pods      []namedCount
}

// reportTemplate is a single self-contained page: the styles are inline and
// there is no JavaScript, so the file can be opened from disk, attached to a
// ticket, or served from a static host without any setup.
var reportTemplate = template.Must(template.New("report").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Pod log report</title>
<style>
:root{--bg:#0b1120;--panel:#141c2f;--text:#e6edf7;--muted:#8fa3bf;--line:#243049;--error:#f87171;--warn:#fbbf24;--info:#38bdf8;--debug:#a78bfa}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--text);font:15px/1.5 ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
main{max-width:900px;margin:0 auto;padding:40px 20px 64px}
h1{font-size:24px;margin:0 0 4px}
.sub{color:var(--muted);font-size:13px;margin-bottom:28px}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(120px,1fr));gap:12px;margin-bottom:28px}
.card{background:var(--panel);border:1px solid var(--line);border-radius:10px;padding:14px}
.card .n{font-size:24px;font-weight:600}
.card .l{color:var(--muted);font-size:12px;text-transform:uppercase;letter-spacing:.04em}
.card.error .n{color:var(--error)}
.card.warn .n{color:var(--warn)}
.card.info .n{color:var(--info)}
.card.debug .n{color:var(--debug)}
section{background:var(--panel);border:1px solid var(--line);border-radius:10px;padding:18px;margin-bottom:20px}
h2{font-size:15px;margin:0 0 14px}
table{width:100%;border-collapse:collapse;font-size:14px}
th,td{text-align:left;padding:8px 6px;border-bottom:1px solid var(--line);vertical-align:top}
th{color:var(--muted);font-weight:500;font-size:12px;text-transform:uppercase}
td.num,th.num{text-align:right;white-space:nowrap;width:64px}
tr:last-child td{border-bottom:0}
.bar{display:grid;grid-template-columns:52px 1fr 40px;gap:10px;align-items:center;margin:6px 0;font-size:13px}
.track{background:#0b1224;border-radius:4px;height:10px;overflow:hidden}
.fill{height:100%;background:linear-gradient(90deg,#2563eb,#38bdf8);border-radius:4px}
.count{text-align:right;color:var(--muted)}
.empty{color:var(--muted);font-size:14px;margin:0}
footer{color:var(--muted);font-size:12px;margin-top:28px}
</style>
</head>
<body>
<main>
<h1>K8s pod log analysis</h1>
<div class="sub">Generated {{.Generated}} &middot; {{.Result.TotalLines}} lines analysed</div>
<div class="cards">
<div class="card"><div class="n">{{.Result.TotalLines}}</div><div class="l">Lines</div></div>
<div class="card error"><div class="n">{{.Result.ErrorCount}}</div><div class="l">Errors</div></div>
<div class="card warn"><div class="n">{{.Result.WarnCount}}</div><div class="l">Warnings</div></div>
<div class="card info"><div class="n">{{.Result.InfoCount}}</div><div class="l">Info</div></div>
<div class="card debug"><div class="n">{{.Result.DebugCount}}</div><div class="l">Debug</div></div>
</div>
<section>
<h2>Top errors</h2>
{{if .Result.TopErrors}}<table>
<thead><tr><th>Message</th><th class="num">Count</th></tr></thead>
<tbody>{{range .Result.TopErrors}}<tr><td>{{.Message}}</td><td class="num">{{.Count}}</td></tr>{{end}}</tbody>
</table>{{else}}<p class="empty">No errors in this log.</p>{{end}}
</section>
{{if .Timeline}}<section>
<h2>Timeline (per minute)</h2>
{{range .Timeline}}<div class="bar"><span>{{.Minute}}</span><div class="track"><div class="fill" style="width:{{.Percent}}%"></div></div><span class="count">{{.Count}}</span></div>{{end}}
</section>{{end}}
{{if .Pods}}<section>
<h2>Lines per pod</h2>
<table>
<thead><tr><th>Pod</th><th class="num">Lines</th></tr></thead>
<tbody>{{range .Pods}}<tr><td>{{.Name}}</td><td class="num">{{.Count}}</td></tr>{{end}}</tbody>
</table>
</section>{{end}}
<footer>Generated by k8s-pod-analyzer {{.Version}}. Errors are grouped after removing timestamps and level prefixes.</footer>
</main>
</body>
</html>
`))

// writeHTML renders a self-contained report. Log lines are untrusted input, so
// the template escapes every value it prints.
func writeHTML(w io.Writer, result AnalysisResult, generated time.Time) error {
	maxCount := 0
	for _, count := range result.Timeline {
		if count > maxCount {
			maxCount = count
		}
	}
	timeline := make([]timelineRow, 0, len(result.Timeline))
	for _, minute := range sortedKeys(result.Timeline) {
		count := result.Timeline[minute]
		percent := 0
		if maxCount > 0 {
			percent = count * 100 / maxCount
		}
		timeline = append(timeline, timelineRow{Minute: minute, Count: count, Percent: percent})
	}

	pods := make([]namedCount, 0, len(result.PodStats))
	for _, pod := range sortedKeys(result.PodStats) {
		pods = append(pods, namedCount{Name: pod, Count: result.PodStats[pod]})
	}
	sort.Slice(pods, func(i, j int) bool {
		if pods[i].Count != pods[j].Count {
			return pods[i].Count > pods[j].Count
		}
		return pods[i].Name < pods[j].Name
	})

	return reportTemplate.Execute(w, htmlReport{
		Generated: generated.Format("2006-01-02 15:04 MST"),
		Version:   versionString(),
		Result:    result,
		Timeline:  timeline,
		Pods:      pods,
	})
}

// renderResult writes the analysis in the requested format. Every command goes
// through this function so text, JSON, CSV and HTML stay in sync.
func renderResult(w io.Writer, result AnalysisResult, format string, timeline, showPods bool) error {
	switch format {
	case "json":
		return writeJSON(w, result)
	case "csv":
		return writeCSV(w, result)
	case "html":
		return writeHTML(w, result, time.Now())
	default:
		return writeText(w, result, timeline, showPods)
	}
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "k8s-pod-analyzer",
		Short: "Kubernetes pod log analyzer with filtering and export",
	}

	var analyzeCmd = &cobra.Command{
		Use:   "analyze [file]",
		Short: "Analyze pod logs from file or stdin",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			level, _ := cmd.Flags().GetString("level")
			outputFmt, _ := cmd.Flags().GetString("output")
			timeline, _ := cmd.Flags().GetBool("timeline")
			var scanner *bufio.Scanner
			if len(args) > 0 && args[0] != "-" {
				f, err := os.Open(args[0])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					os.Exit(1)
				}
				defer f.Close()
				scanner = bufio.NewScanner(f)
			} else {
				scanner = bufio.NewScanner(os.Stdin)
			}
			var entries []LogEntry
			for scanner.Scan() {
				entries = append(entries, parseLogLine(scanner.Text()))
			}
			if err := scanner.Err(); err != nil {
				fmt.Fprintf(os.Stderr, "Error reading logs: %s\n", err)
				os.Exit(1)
			}
			result := analyzeLogs(filterByLevel(entries, level))
			if err := renderResult(os.Stdout, result, outputFmt, timeline, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing %s output: %s\n", outputFmt, err)
				os.Exit(1)
			}
		},
	}
	analyzeCmd.Flags().StringP("level", "l", "", "Filter by level (ERROR, WARN, INFO, DEBUG)")
	analyzeCmd.Flags().StringP("output", "o", "text", "Output format (text, json, csv, html)")
	analyzeCmd.Flags().BoolP("timeline", "t", false, "Show timeline histogram")

	var kubectlCmd = &cobra.Command{
		Use:   "kubectl",
		Short: "Fetch logs directly from Kubernetes",
		Run: func(cmd *cobra.Command, args []string) {
			ns, _ := cmd.Flags().GetString("namespace")
			pod, _ := cmd.Flags().GetString("pod")
			container, _ := cmd.Flags().GetString("container")
			since, _ := cmd.Flags().GetInt("since")
			outputFmt, _ := cmd.Flags().GetString("output")
			level, _ := cmd.Flags().GetString("level")
			timeline, _ := cmd.Flags().GetBool("timeline")

			if pod == "" {
				pods, err := getPodList(ns)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					os.Exit(1)
				}
				if len(pods) == 0 {
					fmt.Println("No pods found")
					return
				}
				fmt.Printf("Found %d pods. Fetching logs...\n\n", len(pods))
				var allEntries []LogEntry
				for _, p := range pods {
					lines, err := getKubectlLogs(ns, p, container, since)
					if err != nil {
						continue
					}
					for _, line := range lines {
						entry := parseLogLine(line)
						entry.Pod = p
						if ns != "" {
							entry.Namespace = ns
						}
						allEntries = append(allEntries, entry)
					}
				}
				result := analyzeLogs(filterByLevel(allEntries, level))
				if err := renderResult(os.Stdout, result, outputFmt, timeline, true); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing %s output: %s\n", outputFmt, err)
					os.Exit(1)
				}
				return
			}

			lines, err := getKubectlLogs(ns, pod, container, since)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
			var entries []LogEntry
			for _, line := range lines {
				entry := parseLogLine(line)
				entry.Pod = pod
				if ns != "" {
					entry.Namespace = ns
				}
				entries = append(entries, entry)
			}
			result := analyzeLogs(filterByLevel(entries, level))
			if err := renderResult(os.Stdout, result, outputFmt, timeline, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing %s output: %s\n", outputFmt, err)
				os.Exit(1)
			}
		},
	}
	kubectlCmd.Flags().StringP("namespace", "n", "", "Kubernetes namespace")
	kubectlCmd.Flags().StringP("pod", "p", "", "Specific pod name")
	kubectlCmd.Flags().StringP("container", "c", "", "Container name")
	kubectlCmd.Flags().IntP("since", "s", 60, "Fetch logs from last N minutes")
	kubectlCmd.Flags().StringP("output", "o", "text", "Output format (text, json, csv, html)")
	kubectlCmd.Flags().StringP("level", "l", "", "Filter by level")
	kubectlCmd.Flags().BoolP("timeline", "t", false, "Show timeline histogram")

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("k8s-pod-analyzer %s\n", versionString())
		},
	}

	rootCmd.AddCommand(analyzeCmd, kubectlCmd, versionCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
