package main

import (
	"testing"
)

func TestFilterByStatus(t *testing.T) {
	results := []AnalysisResult{
		{PodName: "pod1", Status: "Running"},
		{PodName: "pod2", Status: "CrashLoopBackOff"},
		{PodName: "pod3", Status: "Running"},
		{PodName: "pod4", Status: "Error"},
	}

	running := FilterByStatus(results, "Running")
	if len(running) != 2 {
		t.Errorf("Expected 2 running pods, got %d", len(running))
	}

	crashed := FilterByStatus(results, "CrashLoopBackOff")
	if len(crashed) != 1 {
		t.Errorf("Expected 1 CrashLoopBackOff, got %d", len(crashed))
	}

	errorPods := FilterByStatus(results, "Error")
	if len(errorPods) != 1 {
		t.Errorf("Expected 1 Error pod, got %d", len(errorPods))
	}
}

func TestFilterByRestartCount(t *testing.T) {
	results := []AnalysisResult{
		{PodName: "pod1", RestartCount: 0},
		{PodName: "pod2", RestartCount: 5},
		{PodName: "pod3", RestartCount: 10},
		{PodName: "pod4", RestartCount: 3},
	}

	restarts := FilterByRestartCount(results, 5)
	if len(restarts) != 2 {
		t.Errorf("Expected 2 pods with 5+ restarts, got %d", len(restarts))
	}

	high := FilterByRestartCount(results, 10)
	if len(high) != 1 {
		t.Errorf("Expected 1 pod with 10+ restarts, got %d", len(high))
	}
}

func TestFilterByHasErrors(t *testing.T) {
	results := []AnalysisResult{
		{PodName: "pod1", LogErrors: []LogEntry{{Message: "error1"}}},
		{PodName: "pod2", LogErrors: nil},
		{PodName: "pod3", LogErrors: []LogEntry{{Message: "error2"}, {Message: "error3"}}},
		{PodName: "pod4", LogErrors: nil},
	}

	withErrors := FilterByHasErrors(results)
	if len(withErrors) != 2 {
		t.Errorf("Expected 2 pods with errors, got %d", len(withErrors))
	}
}

func TestFilterByNamespace(t *testing.T) {
	results := []AnalysisResult{
		{PodName: "pod1", Namespace: "default"},
		{PodName: "pod2", Namespace: "kube-system"},
		{PodName: "pod3", Namespace: "default"},
		{PodName: "pod4", Namespace: "production"},
	}

	defaultPods := FilterByNamespace(results, "default")
	if len(defaultPods) != 2 {
		t.Errorf("Expected 2 default pods, got %d", len(defaultPods))
	}

	kubePods := FilterByNamespace(results, "kube-system")
	if len(kubePods) != 1 {
		t.Errorf("Expected 1 kube-system pod, got %d", len(kubePods))
	}
}

func TestFilterCombined(t *testing.T) {
	results := []AnalysisResult{
		{PodName: "pod1", Namespace: "default", Status: "Running", RestartCount: 0},
		{PodName: "pod2", Namespace: "default", Status: "CrashLoopBackOff", RestartCount: 10},
		{PodName: "pod3", Namespace: "production", Status: "CrashLoopBackOff", RestartCount: 5},
	}

	filtered := FilterByNamespace(results, "default")
	filtered = FilterByStatus(filtered, "CrashLoopBackOff")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 result, got %d", len(filtered))
	}
	if filtered[0].PodName != "pod2" {
		t.Errorf("Expected pod2, got %s", filtered[0].PodName)
	}
}