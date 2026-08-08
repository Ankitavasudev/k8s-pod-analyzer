package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
}

type AnalysisResult struct {
	TotalLines int
	ErrorCount int
	WarnCount  int
	InfoCount  int
	TopErrors  []string
	Timeline   map[string]int
}

func parseLogLine(line string) LogEntry {
	entry := LogEntry{Message: line}
	re := regexp.MustCompile((\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}))
	if match := re.FindString(line); match != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", match); err == nil {
			entry.Timestamp = t
		}
	}
	upper := strings.ToUpper(line)
	if strings.Contains(upper, "ERROR") || strings.Contains(upper, "FATAL") || strings.Contains(upper, "PANIC") {
		entry.Level = "ERROR"
	} else if strings.Contains(upper, "WARN") {
		entry.Level = "WARN"
	} else {
		entry.Level = "INFO"
	}
	return entry
}

func analyzeLogs(entries []LogEntry) AnalysisResult {
	result := AnalysisResult{TotalLines: len(entries), Timeline: make(map[string]int)}
	errorMsgs := make(map[string]int)
	for _, e := range entries {
		switch e.Level {
		case "ERROR":
			result.ErrorCount++
			msg := e.Message
			if len(msg) > 80 { msg = msg[:80] }
			errorMsgs[msg]++
		case "WARN":
			result.WarnCount++
		default:
			result.InfoCount++
		}
		if !e.Timestamp.IsZero() {
			result.Timeline[e.Timestamp.Format("15:04")]++
		}
	}
	type kv struct { Key string; Value int }
	var sorted []kv
	for k, v := range errorMsgs { sorted = append(sorted, kv{k, v}) }
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Value > sorted[j].Value })
	for i, s := range sorted {
		if i >= 5 { break }
		result.TopErrors = append(result.TopErrors, fmt.Sprintf("[%d times] %s", s.Value, s.Key))
	}
	return result
}

func main() {
	var rootCmd = &cobra.Command{Use: "k8s-pod-analyzer", Short: "Kubernetes pod log analyzer"}
	var analyzeCmd = &cobra.Command{
		Use: "analyze [file]", Short: "Analyze pod logs", Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var scanner *bufio.Scanner
			if len(args) > 0 && args[0] != "-" {
				f, _ := os.Open(args[0]); defer f.Close()
				scanner = bufio.NewScanner(f)
			} else {
				scanner = bufio.NewScanner(os.Stdin)
			}
			var entries []LogEntry
			for scanner.Scan() { entries = append(entries, parseLogLine(scanner.Text())) }
			result := analyzeLogs(entries)
			fmt.Println("=== K8s Pod Log Analysis ===")
			fmt.Printf("Total: %d | Errors: %d | Warnings: %d | Info: %d\n", result.TotalLines, result.ErrorCount, result.WarnCount, result.InfoCount)
			if len(result.TopErrors) > 0 {
				fmt.Println("\nTop Errors:")
				for _, e := range result.TopErrors { fmt.Printf("  - %s\n", e) }
			}
			fmt.Println("\nTimeline:")
			for t, c := range result.Timeline { fmt.Printf("  %s | %s %d\n", t, strings.Repeat("#", c), c) }
		},
	}
	rootCmd.AddCommand(analyzeCmd, &cobra.Command{Use: "version", Run: func(cmd *cobra.Command, args []string) { fmt.Println("v1.0.0") }})
	if err := rootCmd.Execute(); err != nil { fmt.Println(err); os.Exit(1) }
}
