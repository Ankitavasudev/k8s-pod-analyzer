package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type AnalysisResult struct {
	PodName        string            json:"pod_name"
	Namespace      string            json:"namespace"
	Status         string            json:"status"
	RestartCount   int               json:"restart_count"
	NodeName       string            json:"node_name"
	Age            string            json:"age"
	LogErrors      []LogEntry        json:"log_errors,omitempty"
	Timeline       []TimelineEvent   json:"timeline,omitempty"
	SourcePatterns []SourceMatch     json:"source_patterns,omitempty"
	Recommendations []string         json:"recommendations,omitempty"
}

type LogEntry struct {
	Timestamp string json:"timestamp"
	Level     string json:"level"
	Message   string json:"message"
}

type TimelineEvent struct {
	Time    string json:"time"
	Event   string json:"event"
	Reason  string json:"reason,omitempty"
	Message string json:"message,omitempty"
}

type SourceMatch struct {
	Pattern string json:"pattern"
	File    string json:"file"
}

type Report struct {
	GeneratedAt    string            json:"generated_at"
	TotalPods      int               json:"total_pods"
	HealthyPods    int               json:"healthy_pods"
	UnhealthyPods  int               json:"unhealthy_pods"
	Results        []AnalysisResult  json:"results"
	Summary        map[string]int    json:"summary"
}

func OutputJSON(results []AnalysisResult, pretty bool) error {
	report := Report{
		GeneratedAt:   "2026-08-08T12:00:00Z",
		TotalPods:     len(results),
		HealthyPods:   0,
		UnhealthyPods: 0,
		Results:       results,
		Summary:       make(map[string]int),
	}

	for _, r := range results {
		if r.Status == "Running" && r.RestartCount == 0 {
			report.HealthyPods++
		} else {
			report.UnhealthyPods++
		}
		report.Summary[r.Status]++
	}

	var data []byte
	var err error
	if pretty {
		data, err = json.MarshalIndent(report, "", "  ")
	} else {
		data, err = json.Marshal(report)
	}
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func OutputCSV(results []AnalysisResult) error {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	header := []string{"Pod", "Namespace", "Status", "Restarts", "Node", "Age", "Errors", "Recommendations"}
	writer.Write(header)

	for _, r := range results {
		errors := ""
		errMsgs := []string{}
		for _, e := range r.LogErrors {
			errMsgs = append(errMsgs, e.Message)
		}
		if len(errMsgs) > 0 {
			errors = strings.Join(errMsgs, "; ")
		}

		recs := strings.Join(r.Recommendations, "; ")

		record := []string{
			r.PodName,
			r.Namespace,
			r.Status,
			fmt.Sprintf("%d", r.RestartCount),
			r.NodeName,
			r.Age,
			errors,
			recs,
		}
		writer.Write(record)
	}
	return nil
}

func FilterByStatus(results []AnalysisResult, status string) []AnalysisResult {
	var filtered []AnalysisResult
	for _, r := range results {
		if strings.EqualFold(r.Status, status) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func FilterByRestartCount(results []AnalysisResult, minRestarts int) []AnalysisResult {
	var filtered []AnalysisResult
	for _, r := range results {
		if r.RestartCount >= minRestarts {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func FilterByHasErrors(results []AnalysisResult) []AnalysisResult {
	var filtered []AnalysisResult
	for _, r := range results {
		if len(r.LogErrors) > 0 {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func FilterByNamespace(results []AnalysisResult, namespace string) []AnalysisResult {
	var filtered []AnalysisResult
	for _, r := range results {
		if strings.EqualFold(r.Namespace, namespace) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}