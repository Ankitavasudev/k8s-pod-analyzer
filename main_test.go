package main

import (
	"testing"
)

func TestFilterByStatus(t *testing.T) {
	results := []AnalysisResult{
		{PodName: "pod1", Status: "Running"},
		{PodName: "pod2", Status: "CrashLoopBackOff"},
		{PodName: "pod3", Status: "Running"},
	}
	running := FilterByStatus(results, "Running")
	if len(running) != 2 {
		t.Errorf("Expected 2, got %d", len(running))
	}
}

func TestFilterByRestartCount(t *testing.T) {
	results := []AnalysisResult{
		{PodName: "pod1", RestartCount: 0},
		{PodName: "pod2", RestartCount: 10},
	}
	high := FilterByRestartCount(results, 5)
	if len(high) != 1 {
		t.Errorf("Expected 1, got %d", len(high))
	}
}

func TestFilterByHasErrors(t *testing.T) {
	results := []AnalysisResult{
		{PodName: "pod1", LogErrors: []LogEntry{{Message: "err"}}},
		{PodName: "pod2", LogErrors: nil},
	}
	withErrors := FilterByHasErrors(results)
	if len(withErrors) != 1 {
		t.Errorf("Expected 1, got %d", len(withErrors))
	}
}

func TestFilterByNamespace(t *testing.T) {
	results := []AnalysisResult{
		{PodName: "pod1", Namespace: "default"},
		{PodName: "pod2", Namespace: "kube-system"},
	}
	def := FilterByNamespace(results, "default")
	if len(def) != 1 {
		t.Errorf("Expected 1, got %d", len(def))
	}
}

func TestAnalyzePod(t *testing.T) {
	results := analyzePod("./test")
	if len(results) == 0 {
		t.Error("Expected some results")
	}
}

func TestPrintResults(t *testing.T) {
	results := analyzePod("./test")
	printResults(results)
}
