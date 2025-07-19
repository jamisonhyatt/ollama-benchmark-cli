package logs

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"ollama-benchmark/internal/benchmark"
	"ollama-benchmark/internal/i18n"
	"ollama-benchmark/internal/output"
)

func AppendPerformanceLog(results []benchmark.BenchmarkResult) error {
	f, err := os.OpenFile("benchmark.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

// LogEntry represents a parsed log entry
type LogEntry struct {
	Model    string
	Time     float64
	Tokens   int
	TokensPS float64
}

// ParseLogFile reads and parses a benchmark log file
func ParseLogFile(filename string) ([]LogEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", filename, err)
	}
	defer file.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(file)

	// Regex to parse localized log entries
	// Format: [timestamp] key1: value1 | key2: value2 | key3: value3 | key4: value4
	re := regexp.MustCompile(`\[.*?\] ([^:]+): (.*?) \| ([^:]+): ([\d.]+)s \| ([^:]+): (\d+) \| ([^:]+): ([\d.]+)`)

	for scanner.Scan() {
		line := scanner.Text()

		if matches := re.FindStringSubmatch(line); len(matches) == 9 {
			// Parse format: [timestamp] key1: value1 | key2: value2 | key3: value3 | key4: value4
			// matches[2] = model name, matches[4] = time, matches[6] = tokens, matches[8] = tokens/s
			timeVal, _ := strconv.ParseFloat(matches[4], 64)
			tokens, _ := strconv.Atoi(matches[6])
			tokensPS, _ := strconv.ParseFloat(matches[8], 64)

			entries = append(entries, LogEntry{
				Model:    strings.TrimSpace(matches[2]),
				Time:     timeVal,
				Tokens:   tokens,
				TokensPS: tokensPS,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return entries, nil
}

// CompareLogFiles compares two benchmark log files and displays results
func CompareLogFiles(file1, file2 string) error {
	entries1, err := ParseLogFile(file1)
	if err != nil {
		return err
	}

	entries2, err := ParseLogFile(file2)
	if err != nil {
		return err
	}

	// Create maps for easy lookup
	map1 := make(map[string]LogEntry)
	map2 := make(map[string]LogEntry)

	for _, entry := range entries1 {
		map1[entry.Model] = entry
	}

	for _, entry := range entries2 {
		map2[entry.Model] = entry
	}

	// Get all unique models
	allModels := make(map[string]bool)
	for model := range map1 {
		allModels[model] = true
	}
	for model := range map2 {
		allModels[model] = true
	}

	// Print comparison header
	fmt.Printf("\n🔍 %s\n", i18n.T("comparison_title"))
	fmt.Printf("📁 File 1: %s\n", file1)
	fmt.Printf("📁 File 2: %s\n", file2)
	fmt.Println()

	// Prepare data for the table
	var data [][]string

	// Header row
	data = append(data, []string{
		"Model",
		"File1 Token/s",
		"File2 Token/s",
		"File1 Time(s)",
		"File2 Time(s)",
		"Speedup",
	})

	// Data rows
	for model := range allModels {
		entry1, exists1 := map1[model]
		entry2, exists2 := map2[model]

		var speedup string
		var file1TPS, file2TPS, file1Time, file2Time string

		if exists1 && exists2 {
			file1TPS = fmt.Sprintf("%.2f", entry1.TokensPS)
			file2TPS = fmt.Sprintf("%.2f", entry2.TokensPS)
			file1Time = fmt.Sprintf("%.2f", entry1.Time)
			file2Time = fmt.Sprintf("%.2f", entry2.Time)

			if entry1.TokensPS > 0 {
				speedupVal := entry2.TokensPS / entry1.TokensPS
				if speedupVal > 1.1 {
					speedup = fmt.Sprintf("%.2fx ⬆️", speedupVal)
				} else if speedupVal < 0.9 {
					speedup = fmt.Sprintf("%.2fx ⬇️", speedupVal)
				} else {
					speedup = fmt.Sprintf("%.2fx ≈", speedupVal)
				}
			}
		} else if exists1 {
			file1TPS = fmt.Sprintf("%.2f", entry1.TokensPS)
			file2TPS = "N/A"
			file1Time = fmt.Sprintf("%.2f", entry1.Time)
			file2Time = "N/A"
			speedup = "N/A"
		} else if exists2 {
			file1TPS = "N/A"
			file2TPS = fmt.Sprintf("%.2f", entry2.TokensPS)
			file1Time = "N/A"
			file2Time = fmt.Sprintf("%.2f", entry2.Time)
			speedup = "N/A"
		}

		data = append(data, []string{
			model, file1TPS, file2TPS, file1Time, file2Time, speedup,
		})
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header(data[0])
	table.Bulk(data[1:])
	table.Render()

	return nil
}
