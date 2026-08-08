package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type LogLevel int

const (
	LevelInfo LogLevel = iota
	LevelWarn
	LevelError
	LevelFatal
)

func (l LogLevel) String() string {
	return [...]string{"INFO", "WARN", "ERROR", "FATAL"}[l]
}

func (l LogLevel) Color() string {
	return [...]string{"[green]", "[yellow]", "[red]", "[red bold]"}[l]
}

type LogEntry struct {
	Timestamp time.Time
	Level     LogLevel
	Message   string
	Source    string
	Raw       string
}

type AnalysisResult struct {
	TotalLines   int
	ByLevel      map[LogLevel]int
	TopErrors    []Frequency
	Timeline     []TimeBucket
	AvgLineLen   float64
	UniqueSources map[string]int
 Recommendations []string
}

type Frequency struct {
	Text  string
	Count int
}

type TimeBucket struct {
	Time   string
	Errors int
	Warns  int
	Total  int
}

func parseTimestamp(line string) time.Time {
	re := regexp.MustCompile((\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?))
	match := re.FindString(line)
	if match == "" {
		return time.Time{}
	}
	formats := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, match); err == nil {
			return t
		}
	}
	return time.Time{}
}

func classifyLevel(line string) LogLevel {
	upper := strings.ToUpper(line)
	if strings.Contains(upper, "FATAL") || strings.Contains(upper, "PANIC") || strings.Contains(upper, "CRITICAL") {
		return LevelFatal
	}
	if strings.Contains(upper, "ERROR") || strings.Contains(upper, "ERR ") || strings.Contains(upper, "FAILED") {
		return LevelError
	}
	if strings.Contains(upper, "WARN") || strings.Contains(upper, "WARNING") || strings.Contains(upper, "DEPRECATED") {
		return LevelWarn
	}
	return LevelInfo
}

func extractSource(line string) string {
	re := regexp.MustCompile(\[(\w+)\])
	match := re.FindStringSubmatch(line)
	if len(match) > 1 {
		return match[1]
	}
	re2 := regexp.MustCompile(^(\w+[\w.-]+))
	match2 := re2.FindString(line)
	if match2 != "" {
		return match2
	}
	return "unknown"
}

func parseLogLine(line string) LogEntry {
	return LogEntry{
		Timestamp: parseTimestamp(line),
		Level:     classifyLevel(line),
		Message:   line,
		Source:    extractSource(line),
		Raw:       line,
	}
}

func analyzeLogs(entries []LogEntry) AnalysisResult {
	result := AnalysisResult{
		TotalLines:    len(entries),
		ByLevel:       make(map[LogLevel]int),
		UniqueSources: make(map[string]int),
		AvgLineLen:    0,
	}
	errorMsgs := make(map[string]int)
	timeline := make(map[string]*TimeBucket)
	lengths := 0

	for _, e := range entries {
		lengths += len(e.Message)
		result.ByLevel[e.Level]++
		result.UniqueSources[e.Source]++

		if e.Level >= LevelError {
			msg := e.Message
			if len(msg) > 100 {
				msg = msg[:100]
			}
			errorMsgs[msg]++
		}

		if !e.Timestamp.IsZero() {
			key := e.Timestamp.Format("15:04")
			if _, ok := timeline[key]; !ok {
				timeline[key] = &TimeBucket{Time: key}
			}
			timeline[key].Total++
			if e.Level == LevelError || e.Level == LevelFatal {
				timeline[key].Errors++
			}
			if e.Level == LevelWarn {
				timeline[key].Warns++
			}
		}
	}

	if len(entries) > 0 {
		result.AvgLineLen = float64(lengths) / float64(len(entries))
	}

	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range errorMsgs {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Value > sorted[j].Value })
	for i, s := range sorted {
		if i >= 10 {
			break
		}
		result.TopErrors = append(result.TopErrors, Frequency{Text: s.Key, Count: s.Value})
	}

	var timeSorted []TimeBucket
	for _, tb := range timeline {
		timeSorted = append(timeSorted, *tb)
	}
	sort.Slice(timeSorted, func(i, j int) bool { return timeSorted[i].Time < timeSorted[j].Time })
	result.Timeline = timeSorted

	if result.ByLevel[LevelFatal] > 0 {
		result.Recommendations = append(result.Recommendations, "FATAL errors detected — check for crashes and OOMKills")
	}
	if result.ByLevel[LevelError] > 10 {
		result.Recommendations = append(result.Recommendations, "High error rate — review application logs and recent deployments")
	}
	if result.ByLevel[LevelWarn] > 20 {
		result.Recommendations = append(result.Recommendations, "Many warnings — check for deprecated APIs and resource pressure")
	}
	if len(result.UniqueSources) > 5 {
		result.Recommendations = append(result.Recommendations, "Multiple sources — consider structured logging for better filtering")
	}
	if len(result.Recommendations) == 0 {
		result.Recommendations = append(result.Recommendations, "Logs look healthy")
	}

	return result
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "k8s-pod-analyzer",
		Short: "Kubernetes pod log analyzer with error detection and recommendations",
	}

	var analyzeCmd = &cobra.Command{
		Use:   "analyze [file]",
		Short: "Analyze pod logs for errors, warnings, and patterns",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			limit, _ := cmd.Flags().GetInt("limit")
			var scanner *bufio.Scanner

			if len(args) > 0 && args[0] != "-" {
				f, err := os.Open(args[0])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				defer f.Close()
				scanner = bufio.NewScanner(f)
			} else {
				scanner = bufio.NewScanner(os.Stdin)
			}

			scanner.Buffer(make([]byte, 0), 1024*1024)
			var entries []LogEntry
			for scanner.Scan() {
				entries = append(entries, parseLogLine(scanner.Text()))
			}

			result := analyzeLogs(entries)

			fmt.Println("=== K8s Pod Log Analysis ===")
			fmt.Printf("Total Lines:     %d\n", result.TotalLines)
			fmt.Printf("Avg Line Length: %.0f chars\n", result.AvgLineLen)
			fmt.Println()

			fmt.Println("By Level:")
			for level := LevelInfo; level <= LevelFatal; level++ {
				count := result.ByLevel[level]
				if count > 0 {
					bar := strings.Repeat("#", min(count, 50))
					fmt.Printf("  %s %-6s %s %d\n", level.Color(), level, "[/]", count)
				}
			}
			fmt.Println()

			if len(result.TopErrors) > 0 {
				fmt.Printf("Top Errors (showing %d):\n", min(limit, len(result.TopErrors)))
				for i, e := range result.TopErrors {
					if i >= limit {
						break
					}
					fmt.Printf("  [%d times] %s\n", e.Count, e.Text)
				}
				fmt.Println()
			}

			if len(result.UniqueSources) > 0 {
				fmt.Printf("Unique Sources: %d\n", len(result.UniqueSources))
				sources := make([]Frequency, 0, len(result.UniqueSources))
				for k, v := range result.UniqueSources {
					sources = append(sources, Frequency{Text: k, Count: v})
				}
				sort.Slice(sources, func(i, j int) bool { return sources[i].Count > sources[j].Count })
				for i, s := range sources {
					if i >= 5 {
						break
					}
					fmt.Printf("  %s: %d lines\n", s.Text, s.Count)
				}
				fmt.Println()
			}

			if len(result.Timeline) > 0 {
				fmt.Println("Timeline (errors/warns per minute):")
				for _, tb := range result.Timeline {
					if tb.Errors > 0 || tb.Warns > 0 {
						bar := strings.Repeat("E", tb.Errors) + strings.Repeat("W", tb.Warns)
						fmt.Printf("  %s | %s (E:%d W:%d)\n", tb.Time, bar, tb.Errors, tb.Warns)
					}
				}
				fmt.Println()
			}

			fmt.Println("Recommendations:")
			for _, r := range result.Recommendations {
				fmt.Printf("  * %s\n", r)
			}
		},
	}
	analyzeCmd.Flags().IntP("limit", "l", 10, "Max errors to display")

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run:   func(cmd *cobra.Command, args []string) { fmt.Println("k8s-pod-analyzer v1.0.0") },
	}

	rootCmd.AddCommand(analyzeCmd, versionCmd)
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