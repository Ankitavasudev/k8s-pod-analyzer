package main

import (
	"fmt"
	"strings"
)

// FilterOptions contains filter criteria for pods
type FilterOptions struct {
	Namespace    string
	Status       string
	MinRestarts  int
	MaxRestarts  int
	HasErrors    bool
	SearchQuery  string
}

// FilterPods filters pods based on the given options
func FilterPods(pods []PodInfo, opts FilterOptions) []PodInfo {
	var results []PodInfo
	for _, pod := range pods {
		if opts.Namespace != "" && pod.Namespace != opts.Namespace {
			continue
		}
		if opts.Status != "" && pod.Status != opts.Status {
			continue
		}
		if opts.MinRestarts > 0 && pod.Restarts < opts.MinRestarts {
			continue
		}
		if opts.MaxRestarts > 0 && pod.Restarts > opts.MaxRestarts {
			continue
		}
		if opts.SearchQuery != "" && !strings.Contains(strings.ToLower(pod.Name), strings.ToLower(opts.SearchQuery)) {
			continue
		}
		results = append(results, pod)
	}
	return results
}

// FormatPodList formats a list of pods for display
func FormatPodList(pods []PodInfo) string {
	if len(pods) == 0 {
		return "No pods found"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d pod(s):\n\n", len(pods)))
	for _, pod := range pods {
		statusIcon := "?"
		switch pod.Status {
		case "Running":
			statusIcon = "?"
		case "Pending":
			statusIcon = "?"
		case "Failed":
			statusIcon = "?"
		}
		sb.WriteString(fmt.Sprintf("%s %s/%s (restarts: %d)\n", statusIcon, pod.Namespace, pod.Name, pod.Restarts))
		if pod.CPUReq != "" || pod.MemReq != "" {
			sb.WriteString(fmt.Sprintf("   CPU: %s/%s | Memory: %s/%s\n", pod.CPUReq, pod.CPULim, pod.MemReq, pod.MemLim))
		}
	}
	return sb.String()
}

// GetPodStats returns statistics about pods
func GetPodStats(pods []PodInfo) map[string]int {
	stats := map[string]int{
		"total":   len(pods),
		"running": 0,
		"pending": 0,
		"failed":  0,
	}
	for _, pod := range pods {
		switch pod.Status {
		case "Running":
			stats["running"]++
		case "Pending":
			stats["pending"]++
		case "Failed":
			stats["failed"]++
		}
	}
	return stats
}
