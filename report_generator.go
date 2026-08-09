package main

import (
	"fmt"
	"strings"
	"time"
)

type ReportGenerator struct {
	results []AnalysisResult
}

func NewReportGenerator(results []AnalysisResult) *ReportGenerator {
	return &ReportGenerator{results: results}
}

func (g *ReportGenerator) GenerateMarkdown() string {
	var sb strings.Builder

	sb.WriteString("# K8s Pod Analysis Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	summary := g.GetSummary()
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- Total Pods: %d\n", summary["total"]))
	sb.WriteString(fmt.Sprintf("- Healthy: %d\n", summary["healthy"]))
	sb.WriteString(fmt.Sprintf("- Unhealthy: %d\n", summary["unhealthy"]))
	sb.WriteString(fmt.Sprintf("- High Restarts: %d\n\n", summary["high_restarts"]))

	if len(g.results) > 0 {
		sb.WriteString("## Pod Details\n\n")
		for _, r := range g.results {
			sb.WriteString(fmt.Sprintf("### %s/%s\n\n", r.Namespace, r.PodName))
			sb.WriteString(fmt.Sprintf("- **Status:** %s\n", r.Status))
			sb.WriteString(fmt.Sprintf("- **Restarts:** %d\n", r.RestartCount))
			sb.WriteString(fmt.Sprintf("- **Node:** %s\n", r.NodeName))
			sb.WriteString(fmt.Sprintf("- **Age:** %s\n\n", r.Age))

			if len(r.LogErrors) > 0 {
				sb.WriteString("**Errors:**\n")
				for _, e := range r.LogErrors {
					sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", e.Timestamp, e.Level, e.Message))
				}
				sb.WriteString("\n")
			}

			if len(r.Recommendations) > 0 {
				sb.WriteString("**Recommendations:**\n")
				for _, rec := range r.Recommendations {
					sb.WriteString(fmt.Sprintf("- %s\n", rec))
				}
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

func (g *ReportGenerator) GenerateHTML() string {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	sb.WriteString("<title>K8s Pod Analysis Report</title>\n")
	sb.WriteString("<style>\n")
	sb.WriteString("body { font-family: system-ui; max-width: 1200px; margin: 0 auto; padding: 20px; }\n")
	sb.WriteString("table { width: 100%; border-collapse: collapse; }\n")
	sb.WriteString("th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }\n")
	sb.WriteString("th { background: #f5f5f5; }\n")
	sb.WriteString(".healthy { color: green; }\n")
	sb.WriteString(".unhealthy { color: red; }\n")
	sb.WriteString("</style>\n</head>\n<body>\n")

	sb.WriteString("<h1>K8s Pod Analysis Report</h1>\n")
	sb.WriteString(fmt.Sprintf("<p>Generated: %s</p>\n", time.Now().Format("2006-01-02 15:04:05")))

	summary := g.GetSummary()
	sb.WriteString("<h2>Summary</h2>\n")
	sb.WriteString("<ul>\n")
	sb.WriteString(fmt.Sprintf("<li>Total Pods: %d</li>\n", summary["total"]))
	sb.WriteString(fmt.Sprintf("<li>Healthy: %d</li>\n", summary["healthy"]))
	sb.WriteString(fmt.Sprintf("<li>Unhealthy: %d</li>\n", summary["unhealthy"]))
	sb.WriteString("</ul>\n")

	sb.WriteString("<h2>Pod Details</h2>\n")
	sb.WriteString("<table>\n")
	sb.WriteString("<tr><th>Pod</th><th>Namespace</th><th>Status</th><th>Restarts</th><th>Node</th><th>Age</th></tr>\n")

	for _, r := range g.results {
		statusClass := "healthy"
		if r.Status != "Running" || r.RestartCount > 0 {
			statusClass = "unhealthy"
		}
		sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td class=\"%s\">%s</td><td>%d</td><td>%s</td><td>%s</td></tr>\n",
			r.PodName, r.Namespace, statusClass, r.Status, r.RestartCount, r.NodeName, r.Age))
	}

	sb.WriteString("</table>\n</body>\n</html>")
	return sb.String()
}

func (g *ReportGenerator) GetSummary() map[string]int {
	summary := map[string]int{
		"total":         len(g.results),
		"healthy":       0,
		"unhealthy":     0,
		"high_restarts": 0,
	}

	for _, r := range g.results {
		if r.Status == "Running" && r.RestartCount == 0 {
			summary["healthy"]++
		} else {
			summary["unhealthy"]++
		}
		if r.RestartCount > 5 {
			summary["high_restarts"]++
		}
	}

	return summary
}