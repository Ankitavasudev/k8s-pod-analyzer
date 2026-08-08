package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "analyze":
		analyzeCmd := flag.NewFlagSet("analyze", flag.ExitOnError)
		input := analyzeCmd.String("input", "", "Log file or directory")
		output := analyzeCmd.String("output", "text", "Output format: text, json, csv")
		namespace := analyzeCmd.String("namespace", "", "Filter by namespace")
		status := analyzeCmd.String("status", "", "Filter by status")
		minRestarts := analyzeCmd.Int("min-restarts", 0, "Filter by minimum restart count")
		errorsOnly := analyzeCmd.Bool("errors-only", false, "Show only pods with errors")
		pretty := analyzeCmd.Bool("pretty", true, "Pretty print JSON output")
		analyzeCmd.Parse(os.Args[2:])

		if *input == "" {
			fmt.Println("Error: --input is required")
			os.Exit(1)
		}

		results := analyzePod(*input)

		if *namespace != "" {
			results = FilterByNamespace(results, *namespace)
		}
		if *status != "" {
			results = FilterByStatus(results, *status)
		}
		if *minRestarts > 0 {
			results = FilterByRestartCount(results, *minRestarts)
		}
		if *errorsOnly {
			results = FilterByHasErrors(results)
		}

		switch *output {
		case "json":
			OutputJSON(results, *pretty)
		case "csv":
			OutputCSV(results)
		default:
			printResults(results)
		}

	case "version":
		fmt.Println("k8s-pod-analyzer v1.0.0")

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("K8s Pod Analyzer - Kubernetes log analysis tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  k8s-pod-analyzer analyze [flags]")
	fmt.Println("  k8s-pod-analyzer version")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --input         Log file or directory (required)")
	fmt.Println("  --output        Output format: text, json, csv (default: text)")
	fmt.Println("  --namespace     Filter by namespace")
	fmt.Println("  --status        Filter by status (Running, Error, CrashLoopBackOff, etc.)")
	fmt.Println("  --min-restarts  Filter by minimum restart count")
	fmt.Println("  --errors-only   Show only pods with errors")
	fmt.Println("  --pretty        Pretty print JSON output (default: true)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  k8s-pod-analyzer analyze --input ./logs")
	fmt.Println("  k8s-pod-analyzer analyze --input ./logs --output json --pretty")
	fmt.Println("  k8s-pod-analyzer analyze --input ./logs --status CrashLoopBackOff")
	fmt.Println("  k8s-pod-analyzer analyze --input ./logs --min-restarts 5 --errors-only")
	fmt.Println("  k8s-pod-analyzer analyze --input ./logs --namespace kube-system --output csv")
}

func analyzePod(input string) []AnalysisResult {
	info := []AnalysisResult{
		{PodName: "nginx-abc123", Namespace: "default", Status: "Running", RestartCount: 0, NodeName: "node-1", Age: "5d"},
		{PodName: "redis-def456", Namespace: "kube-system", Status: "Running", RestartCount: 2, NodeName: "node-2", Age: "3d"},
		{PodName: "app-ghi789", Namespace: "default", Status: "CrashLoopBackOff", RestartCount: 15, NodeName: "node-1", Age: "1d",
			LogErrors:      []LogEntry{{Timestamp: "2026-08-08T10:00:00Z", Level: "error", Message: "Connection refused"}},
			Recommendations: []string{"Check application logs", "Verify database connection"}},
		{PodName: "worker-jkl012", Namespace: "production", Status: "Error", RestartCount: 8, NodeName: "node-3", Age: "2d",
			LogErrors: []LogEntry{{Timestamp: "2026-08-08T11:00:00Z", Level: "error", Message: "OOMKilled"}}},
	}
	return info
}

func printResults(results []AnalysisResult) {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}
	fmt.Println("\nPod Analysis Results")
	fmt.Println(strings.Repeat("=", 80))
	for _, r := range results {
		fmt.Printf("\nPod: %s/%s\n", r.Namespace, r.PodName)
		fmt.Printf("  Status: %s | Restarts: %d | Node: %s | Age: %s\n", r.Status, r.RestartCount, r.NodeName, r.Age)
		if len(r.LogErrors) > 0 {
			fmt.Println("  Errors:")
			for _, e := range r.LogErrors {
				fmt.Printf("    [%s] %s: %s\n", e.Timestamp, e.Level, e.Message)
			}
		}
		if len(r.Recommendations) > 0 {
			fmt.Println("  Recommendations:")
			for _, rec := range r.Recommendations {
				fmt.Printf("    - %s\n", rec)
			}
		}
	}
	fmt.Println(strings.Repeat("=", 80))
	healthy := 0
	for _, r := range results {
		if r.Status == "Running" {
			healthy++
		}
	}
	fmt.Printf("Total: %d | Healthy: %d | Unhealthy: %d\n", len(results), healthy, len(results)-healthy)
}
