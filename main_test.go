package main

import (
	"testing"
)

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

func TestOutputJSON(t *testing.T) {
	results := analyzePod("./test")
	err := OutputJSON(results, true)
	if err != nil {
		t.Errorf("OutputJSON failed: %v", err)
	}
}

func TestOutputCSV(t *testing.T) {
	results := analyzePod("./test")
	err := OutputCSV(results)
	if err != nil {
		t.Errorf("OutputCSV failed: %v", err)
	}
}