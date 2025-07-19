package logs

import (
	"fmt"
	"os"
	"testing"
	"time"

	"ollama-benchmark/internal/benchmark"
	"ollama-benchmark/internal/i18n"
	"ollama-benchmark/internal/output"
)

func TestAppendAndParsePerformanceLog(t *testing.T) {
	// Test data
	testResults := []benchmark.BenchmarkResult{
		{
			Model:     "test-model-1:7b",
			Prompt:    "Test prompt 1",
			Trial:     1,
			Duration:  time.Second * 5,
			Tokens:    100,
			TokenPerS: 20.0,
		},
		{
			Model:     "test-model-2:13b",
			Prompt:    "Test prompt 2",
			Trial:     1,
			Duration:  time.Second * 10,
			Tokens:    200,
			TokenPerS: 20.0,
		},
	}

	tests := []struct {
		name     string
		language string
		logFile  string
	}{
		{
			name:     "English logs",
			language: "en",
			logFile:  "test_benchmark_en.log",
		},
		{
			name:     "Turkish logs",
			language: "tr",
			logFile:  "test_benchmark_tr.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing test file
			os.Remove(tt.logFile)
			defer os.Remove(tt.logFile)

			// Load the language
			err := i18n.Load(tt.language)
			if err != nil {
				t.Fatalf("Failed to load language %s: %v", tt.language, err)
			}

			// Temporarily override the log file by creating a custom version
			// We'll create a test-specific AppendPerformanceLog that writes to our test file
			err = appendPerformanceLogToFile(tt.logFile, testResults)
			if err != nil {
				t.Fatalf("Failed to write log entries: %v", err)
			}

			// Parse the log file
			entries, err := ParseLogFile(tt.logFile)
			if err != nil {
				t.Fatalf("Failed to parse log file: %v", err)
			}

			// Since AppendPerformanceLog aggregates results, we need to check against aggregated data
			// For our test data with same models, we should get aggregated results
			expectedEntries := 2 // We have 2 different models

			if len(entries) != expectedEntries {
				t.Errorf("Expected %d entries, got %d", expectedEntries, len(entries))
			}

			// Validate that we can parse what we wrote
			for _, entry := range entries {
				if entry.Model == "" {
					t.Error("Parsed entry has empty model")
				}
				if entry.Time <= 0 {
					t.Error("Parsed entry has invalid time")
				}
				if entry.Tokens <= 0 {
					t.Error("Parsed entry has invalid tokens")
				}
				if entry.TokensPS <= 0 {
					t.Error("Parsed entry has invalid tokens per second")
				}
			}
		})
	}
}

// Helper function that mimics AppendPerformanceLog but writes to a specific file
func appendPerformanceLogToFile(filename string, results []benchmark.BenchmarkResult) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf(i18n.T("err_file_log"), err)
	}
	defer f.Close()

	aggregated := output.Aggregate(results)

	for _, r := range aggregated {
		entry := fmt.Sprintf(
			"[%s] %s: %s | %s: %.2fs | %s: %d | %s: %.2f\n",
			time.Now().Format(time.RFC3339),
			i18n.T("header_model"), r.Model,
			i18n.T("header_time"), r.AvgDuration.Seconds(),
			i18n.T("header_tokens"), r.TotalTokens,
			i18n.T("header_tps"), r.TokenPerS,
		)
		f.WriteString(entry)
	}

	return nil
}

func TestParseLogFileInvalidFormat(t *testing.T) {
	// Create a test file with invalid format
	testFile := "test_invalid.log"
	defer os.Remove(testFile)

	content := `[2025-07-19T21:18:37Z] This is not a valid log format
[2025-07-19T21:18:37Z] Model: test | InvalidFormat
Some random text
`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	entries, err := ParseLogFile(testFile)
	if err != nil {
		t.Fatalf("ParseLogFile should not fail on invalid lines: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries for invalid format, got %d", len(entries))
	}
}

func TestParseLogFileNotFound(t *testing.T) {
	_, err := ParseLogFile("nonexistent.log")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestCompareLogFiles(t *testing.T) {
	// Load English for consistent test output
	err := i18n.Load("en")
	if err != nil {
		t.Fatalf("Failed to load English language: %v", err)
	}

	// Create two test log files
	file1 := "test_compare_1.log"
	file2 := "test_compare_2.log"
	defer os.Remove(file1)
	defer os.Remove(file2)

	content1 := `[2025-07-19T21:18:37Z] Model: test-model-1 | Time (s): 5.00s | Tokens: 100 | Token/s: 20.00
[2025-07-19T21:18:37Z] Model: test-model-2 | Time (s): 10.00s | Tokens: 200 | Token/s: 20.00
`

	content2 := `[2025-07-19T21:18:37Z] Model: test-model-1 | Time (s): 4.00s | Tokens: 100 | Token/s: 25.00
[2025-07-19T21:18:37Z] Model: test-model-3 | Time (s): 8.00s | Tokens: 160 | Token/s: 20.00
`

	err = os.WriteFile(file1, []byte(content1), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}

	err = os.WriteFile(file2, []byte(content2), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}

	// Test comparison (this will print to stdout, which is expected in tests)
	err = CompareLogFiles(file1, file2)
	if err != nil {
		t.Errorf("CompareLogFiles failed: %v", err)
	}
}

func TestCompareLogFilesErrors(t *testing.T) {
	err := i18n.Load("en")
	if err != nil {
		t.Fatalf("Failed to load English language: %v", err)
	}

	// Test with nonexistent files
	err = CompareLogFiles("nonexistent1.log", "nonexistent2.log")
	if err == nil {
		t.Error("Expected error for nonexistent files, got nil")
	}

	// Test with one valid and one invalid file
	validFile := "test_valid.log"
	defer os.Remove(validFile)

	content := `[2025-07-19T21:18:37Z] Model: test-model | Time (s): 5.00s | Tokens: 100 | Token/s: 20.00
`
	err = os.WriteFile(validFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create valid test file: %v", err)
	}

	err = CompareLogFiles(validFile, "nonexistent.log")
	if err == nil {
		t.Error("Expected error for nonexistent second file, got nil")
	}
}
