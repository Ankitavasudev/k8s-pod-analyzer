package main

import "fmt"

func GetStats(pods []PodInfo) map[string]int {
	stats := map[string]int{"total": len(pods), "running": 0, "pending": 0, "failed": 0}
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

func PrintStats(pods []PodInfo) {
	stats := GetStats(pods)
	fmt.Printf("Total: %d | Running: %d | Pending: %d | Failed: %d\n",
		stats["total"], stats["running"], stats["pending"], stats["failed"])
}