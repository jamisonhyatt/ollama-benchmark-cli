package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"ollama-benchmark/internal/benchmark"
	"ollama-benchmark/internal/i18n"

	"github.com/olekukonko/tablewriter"
)

// AggregatedResult holds the aggregated benchmark results for a model.
type AggregatedResult struct {
	Model       string        // Model name
	TotalTime   time.Duration // Total time taken for all trials
	AvgDuration time.Duration // Average time per trial
	TotalTokens int           // Total number of tokens processed
	TokenPerS   float64       // Tokens processed per second
}

// FormatAsTable displays the benchmark results in a tabular format.
func FormatAsTable(results []benchmark.BenchmarkResult, tokensOnly bool) {
	aggregated := Aggregate(results)

	sort.Slice(aggregated, func(i, j int) bool {
		return aggregated[i].TokenPerS > aggregated[j].TokenPerS
	})

	// Prepare data for the table
	var data [][]string

	// Header row
	data = append(data, []string{
		i18n.T("header_model"),
		"Total Time (s)",
		"Avg Time (s)",
		i18n.T("header_tokens"),
		i18n.T("header_tps"),
	})

	// Data rows
	for _, r := range aggregated {
		data = append(data, []string{
			r.Model,
			fmt.Sprintf("%.2f", r.TotalTime.Seconds()),
			fmt.Sprintf("%.2f", r.AvgDuration.Seconds()),
			fmt.Sprintf("%d", r.TotalTokens),
			fmt.Sprintf("%.1f", r.TokenPerS),
		})
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header(data[0])
	table.Bulk(data[1:])
	table.Render()
}

// PrintDetails outputs the detailed results of each benchmark trial.
func PrintDetails(results []benchmark.BenchmarkResult) {
	fmt.Println("\n" + i18n.T("details_title"))
	for _, r := range results {
		promptDisplay := r.PromptName
		if promptDisplay == "" {
			// Fallback to truncated prompt if no name is available
			if len(r.Prompt) > 50 {
				promptDisplay = r.Prompt[:47] + "..."
			} else {
				promptDisplay = r.Prompt
			}
		}
		fmt.Printf("[%s] Trial %d | %s: %s\n", r.Model, r.Trial, i18n.T("field_prompt"), promptDisplay)
		fmt.Printf("  ➜ %s: %d | %s: %.2fs | %s: %.2f\n\n",
			i18n.T("field_tokens"), r.Tokens,
			i18n.T("field_time"), r.Duration.Seconds(),
			i18n.T("field_tps"), r.TokenPerS)
	}
}

// GroupByModel organizes the benchmark results by model.
func GroupByModel(results []benchmark.BenchmarkResult) map[string][]benchmark.BenchmarkResult {
	m := make(map[string][]benchmark.BenchmarkResult)
	for _, r := range results {
		m[r.Model] = append(m[r.Model], r)
	}
	return m
}

// WriteJSON saves the benchmark results in JSON format to the specified file.
func WriteJSON(path string, results []benchmark.BenchmarkResult) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// WriteCSV saves the benchmark results in CSV format to the specified file.
func WriteCSV(path string, results []benchmark.BenchmarkResult) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	headers := []string{"model", "prompt", "trial", "duration_s", "tokens", "token_per_s"}
	writer.Write(headers)

	for _, r := range results {
		record := []string{
			r.Model,
			r.Prompt,
			fmt.Sprintf("%d", r.Trial),
			fmt.Sprintf("%.2f", r.Duration.Seconds()),
			fmt.Sprintf("%d", r.Tokens),
			fmt.Sprintf("%.1f", r.TokenPerS),
		}
		writer.Write(record)
	}

	return nil
}

// WriteTXT saves the benchmark results in a plain text format to the specified file.
func WriteTXT(path string, results []benchmark.BenchmarkResult, tokensOnly bool) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintln(file, i18n.T("summary_title"))
	fmt.Fprintf(file, "%s\t%s\t%s\t%s\n",
		i18n.T("header_model"), i18n.T("header_time"), i18n.T("header_tokens"), i18n.T("header_tps"))

	for _, r := range Aggregate(results) {
		fmt.Fprintf(file, "%s\t%.2f\t%d\t%.1f\n", r.Model, r.AvgDuration.Seconds(), r.TotalTokens, r.TokenPerS)
	}

	fmt.Fprintln(file, "\n"+i18n.T("details_title"))
	for _, r := range results {
		fmt.Fprintf(file, "[%s] Trial %d | %s: %q\n", r.Model, r.Trial, i18n.T("field_prompt"), r.Prompt)
		fmt.Fprintf(file, "  ➜ %s: %d | %s: %.2fs | %s: %.2f\n\n",
			i18n.T("field_tokens"), r.Tokens,
			i18n.T("field_time"), r.Duration.Seconds(),
			i18n.T("field_tps"), r.TokenPerS)
	}

	return nil
}

// ShowComparison displays a comparison of the benchmark results across different models.
func ShowComparison(results []benchmark.BenchmarkResult) {
	agg := Aggregate(results)

	sort.Slice(agg, func(i, j int) bool {
		return agg[i].TokenPerS > agg[j].TokenPerS
	})

	fmt.Println()
	fmt.Println("🔍 " + i18n.T("comparison_title"))
	fmt.Println()

	// Prepare data for the table
	var data [][]string

	// Header row
	data = append(data, []string{
		i18n.T("header_model"),
		"Total Time (s)",
		"Avg Time (s)",
		i18n.T("header_tokens"),
		i18n.T("header_tps"),
		i18n.T("header_rank"),
	})

	// Data rows with rankings
	for i, r := range agg {
		rank := "-"
		switch i {
		case 0:
			rank = "🥇"
		case 1:
			rank = "🥈"
		case 2:
			rank = "🥉"
		}

		data = append(data, []string{
			r.Model,
			fmt.Sprintf("%.2f", r.TotalTime.Seconds()),
			fmt.Sprintf("%.2f", r.AvgDuration.Seconds()),
			fmt.Sprintf("%d", r.TotalTokens),
			fmt.Sprintf("%.1f", r.TokenPerS),
			rank,
		})
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header(data[0])
	table.Bulk(data[1:])
	table.Render()
}

// Aggregate consolidates the benchmark results for each model.
func Aggregate(results []benchmark.BenchmarkResult) []AggregatedResult {
	grouped := make(map[string][]benchmark.BenchmarkResult)

	for _, r := range results {
		grouped[r.Model] = append(grouped[r.Model], r)
	}

	var aggregated []AggregatedResult
	for model, group := range grouped {
		var totalTime time.Duration
		var totalTokens int

		for _, r := range group {
			totalTime += r.Duration
			totalTokens += r.Tokens
		}

		count := len(group)
		// Use total time instead of average time to match the live timer
		tps := float64(totalTokens) / totalTime.Seconds()

		aggregated = append(aggregated, AggregatedResult{
			Model:       model,
			TotalTime:   totalTime,
			AvgDuration: totalTime / time.Duration(count),
			TotalTokens: totalTokens,
			TokenPerS:   tps,
		})
	}

	return aggregated
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, os.ModePerm)
}
