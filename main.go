package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
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
	TotalLines  int              `json:"total_lines"`
	ErrorCount  int              `json:"error_count"`
	WarnCount   int              `json:"warn_count"`
	InfoCount   int              `json:"info_count"`
	DebugCount  int              `json:"debug_count"`
	TopErrors   []ErrorCount     `json:"top_errors"`
	Timeline    map[string]int   `json:"timeline"`
	LevelCounts map[string]int   `json:"level_counts"`
	PodStats    map[string]int   `json:"pod_stats,omitempty"`
	NamespaceStats map[string]int `json:"namespace_stats,omitempty"`
}

type ErrorCount struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

type Report struct {
	Timestamp   string          `json:"timestamp"`
	Summary     AnalysisResult  `json:"summary"`
	Entries     []LogEntry      `json:"entries,omitempty"`
}

func parseLogLine(line string) LogEntry {
	entry := LogEntry{Message: line}
	re := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2})`)
	if match := re.FindString(line); match != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", match); err == nil {
			entry.Timestamp = t
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

func analyzeLogs(entries []LogEntry) AnalysisResult {
	result := AnalysisResult{
		TotalLines:  len(entries),
		Timeline:    make(map[string]int),
		LevelCounts: make(map[string]int),
		PodStats:    make(map[string]int),
		NamespaceStats: make(map[string]int),
	}
	errorMsgs := make(map[string]int)
	for _, e := range entries {
		result.LevelCounts[e.Level]++
		switch e.Level {
		case "ERROR":
			result.ErrorCount++
			msg := e.Message
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
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Value > sorted[j].Value })
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

func outputJSON(result AnalysisResult) {
	report := Report{
		Timestamp: time.Now().Format(time.RFC3339),
		Summary:   result,
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(data))
}

func outputCSV(result AnalysisResult) {
	writer := csv.NewWriter(os.Stdout)
	writer.Write([]string{"metric", "value"})
	writer.Write([]string{"total_lines", strconv.Itoa(result.TotalLines)})
	writer.Write([]string{"error_count", strconv.Itoa(result.ErrorCount)})
	writer.Write([]string{"warn_count", strconv.Itoa(result.WarnCount)})
	writer.Write([]string{"info_count", strconv.Itoa(result.InfoCount)})
	writer.Write([]string{"debug_count", strconv.Itoa(result.DebugCount)})
	writer.Write([]string{})
	writer.Write([]string{"timestamp", "count"})
	for t, c := range result.Timeline {
		writer.Write([]string{t, strconv.Itoa(c)})
	}
	writer.Write([]string{})
	writer.Write([]string{"error_message", "count"})
	for _, e := range result.TopErrors {
		writer.Write([]string{e.Message, strconv.Itoa(e.Count)})
	}
	writer.Flush()
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
			if level != "" {
				level = strings.ToUpper(level)
				var filtered []LogEntry
				for _, e := range entries {
					if e.Level == level {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}
			result := analyzeLogs(entries)
			switch outputFmt {
			case "json":
				outputJSON(result)
			case "csv":
				outputCSV(result)
			default:
				fmt.Printf("=== K8s Pod Log Analysis ===\n")
				fmt.Printf("Total: %d | Errors: %d | Warnings: %d | Info: %d | Debug: %d\n",
					result.TotalLines, result.ErrorCount, result.WarnCount, result.InfoCount, result.DebugCount)
				if len(result.TopErrors) > 0 {
					fmt.Println("\nTop Errors:")
					for _, e := range result.TopErrors {
						fmt.Printf("  [%d times] %s\n", e.Count, e.Message)
					}
				}
				if timeline {
					fmt.Println("\nTimeline:")
					var times []string
					for t := range result.Timeline {
						times = append(times, t)
					}
					sort.Strings(times)
					for _, t := range times {
						c := result.Timeline[t]
						bar := strings.Repeat("#", min(c, 50))
						fmt.Printf("  %s | %s %d\n", t, bar, c)
					}
				}
			}
		},
	}
	analyzeCmd.Flags().StringP("level", "l", "", "Filter by level (ERROR, WARN, INFO, DEBUG)")
	analyzeCmd.Flags().StringP("output", "o", "text", "Output format (text, json, csv)")
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
				if level != "" {
					level = strings.ToUpper(level)
					var filtered []LogEntry
					for _, e := range allEntries {
						if e.Level == level {
							filtered = append(filtered, e)
						}
					}
					allEntries = filtered
				}
				result := analyzeLogs(allEntries)
				switch outputFmt {
				case "json":
					outputJSON(result)
				case "csv":
					outputCSV(result)
				default:
					fmt.Printf("=== K8s Pod Log Analysis ===\n")
					fmt.Printf("Total: %d | Errors: %d | Warnings: %d | Info: %d | Debug: %d\n",
						result.TotalLines, result.ErrorCount, result.WarnCount, result.InfoCount, result.DebugCount)
					if len(result.PodStats) > 0 {
						fmt.Println("\nPod Breakdown:")
						for pod, count := range result.PodStats {
							fmt.Printf("  %s: %d lines\n", pod, count)
						}
					}
					if len(result.TopErrors) > 0 {
						fmt.Println("\nTop Errors:")
						for _, e := range result.TopErrors {
							fmt.Printf("  [%d times] %s\n", e.Count, e.Message)
						}
					}
					if timeline {
						fmt.Println("\nTimeline:")
						var times []string
						for t := range result.Timeline {
							times = append(times, t)
						}
						sort.Strings(times)
						for _, t := range times {
							c := result.Timeline[t]
							bar := strings.Repeat("#", min(c, 50))
							fmt.Printf("  %s | %s %d\n", t, bar, c)
						}
					}
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
			if level != "" {
				level = strings.ToUpper(level)
				var filtered []LogEntry
				for _, e := range entries {
					if e.Level == level {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}
			result := analyzeLogs(entries)
			switch outputFmt {
			case "json":
				outputJSON(result)
			case "csv":
				outputCSV(result)
			default:
				fmt.Printf("=== K8s Pod Log Analysis ===\n")
				fmt.Printf("Total: %d | Errors: %d | Warnings: %d | Info: %d | Debug: %d\n",
					result.TotalLines, result.ErrorCount, result.WarnCount, result.InfoCount, result.DebugCount)
				if len(result.TopErrors) > 0 {
					fmt.Println("\nTop Errors:")
					for _, e := range result.TopErrors {
						fmt.Printf("  [%d times] %s\n", e.Count, e.Message)
					}
				}
				if timeline {
					fmt.Println("\nTimeline:")
					var times []string
					for t := range result.Timeline {
						times = append(times, t)
					}
					sort.Strings(times)
					for _, t := range times {
						c := result.Timeline[t]
						bar := strings.Repeat("#", min(c, 50))
						fmt.Printf("  %s | %s %d\n", t, bar, c)
					}
				}
			}
		},
	}
	kubectlCmd.Flags().StringP("namespace", "n", "", "Kubernetes namespace")
	kubectlCmd.Flags().StringP("pod", "p", "", "Specific pod name")
	kubectlCmd.Flags().StringP("container", "c", "", "Container name")
	kubectlCmd.Flags().IntP("since", "s", 60, "Fetch logs from last N minutes")
	kubectlCmd.Flags().StringP("output", "o", "text", "Output format (text, json, csv)")
	kubectlCmd.Flags().StringP("level", "l", "", "Filter by level")
	kubectlCmd.Flags().BoolP("timeline", "t", false, "Show timeline histogram")

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("k8s-pod-analyzer v2.0.0")
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
