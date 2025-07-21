package output

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"ollama-benchmark/internal/benchmark"
	"ollama-benchmark/internal/i18n"

	"github.com/olekukonko/tablewriter"
)

// ComparisonResult holds the comparison data between two benchmark runs
type ComparisonResult struct {
	Run1         BenchmarkExecution `json:"run1"`
	Run2         BenchmarkExecution `json:"run2"`
	ModelDeltas  []ModelDelta       `json:"model_deltas"`
	PromptDeltas []PromptDelta      `json:"prompt_deltas"`
	HardwareDiff HardwareDiff       `json:"hardware_diff"`
}

// ModelDelta represents performance difference for a model between two runs
type ModelDelta struct {
	Model              string  `json:"model"`
	Run1TokensPerSec   float64 `json:"run1_tokens_per_sec"`
	Run2TokensPerSec   float64 `json:"run2_tokens_per_sec"`
	TokensPerSecDelta  float64 `json:"tokens_per_sec_delta"`
	TokensPerSecChange float64 `json:"tokens_per_sec_change_percent"`
	Run1AvgTime        float64 `json:"run1_avg_time"`
	Run2AvgTime        float64 `json:"run2_avg_time"`
	AvgTimeDelta       float64 `json:"avg_time_delta"`
	AvgTimeChange      float64 `json:"avg_time_change_percent"`
	PresentInBoth      bool    `json:"present_in_both"`
}

// PromptDelta represents performance difference for a prompt between two runs
type PromptDelta struct {
	PromptChecksum    string             `json:"prompt_checksum"`
	PromptText        string             `json:"prompt_text"`
	ModelPerformances []PromptModelDelta `json:"model_performances"`
}

// PromptModelDelta represents how a specific model performed on a prompt between runs
type PromptModelDelta struct {
	Model              string  `json:"model"`
	Run1TokensPerSec   float64 `json:"run1_tokens_per_sec"`
	Run2TokensPerSec   float64 `json:"run2_tokens_per_sec"`
	TokensPerSecDelta  float64 `json:"tokens_per_sec_delta"`
	TokensPerSecChange float64 `json:"tokens_per_sec_change_percent"`
	PresentInBoth      bool    `json:"present_in_both"`
}

// HardwareDiff represents differences in hardware between two runs
type HardwareDiff struct {
	OSChanged       bool      `json:"os_changed"`
	Run1OS          string    `json:"run1_os"`
	Run2OS          string    `json:"run2_os"`
	ArchChanged     bool      `json:"arch_changed"`
	Run1Arch        string    `json:"run1_arch"`
	Run2Arch        string    `json:"run2_arch"`
	CPUCountChanged bool      `json:"cpu_count_changed"`
	Run1CPUCount    int       `json:"run1_cpu_count"`
	Run2CPUCount    int       `json:"run2_cpu_count"`
	CloudChanged    bool      `json:"cloud_changed"`
	Run1Cloud       CloudInfo `json:"run1_cloud"`
	Run2Cloud       CloudInfo `json:"run2_cloud"`
	GPUChanged      bool      `json:"gpu_changed"`
	Run1GPUs        []GPUInfo `json:"run1_gpus"`
	Run2GPUs        []GPUInfo `json:"run2_gpus"`
}

// CompareBenchmarkFiles compares two enhanced JSON benchmark files and displays the results
func CompareBenchmarkFiles(file1Path, file2Path string) error {
	// Read and parse first file
	run1, err := readBenchmarkFile(file1Path)
	if err != nil {
		return fmt.Errorf("failed to read first benchmark file: %w", err)
	}

	// Read and parse second file
	run2, err := readBenchmarkFile(file2Path)
	if err != nil {
		return fmt.Errorf("failed to read second benchmark file: %w", err)
	}

	// Create comparison result
	comparison := createComparison(run1, run2)

	// Display comparison results
	displayComparison(comparison, file1Path, file2Path)

	return nil
}

// readBenchmarkFile reads and parses an enhanced JSON benchmark file
func readBenchmarkFile(filePath string) (BenchmarkExecution, error) {
	var execution BenchmarkExecution

	data, err := os.ReadFile(filePath)
	if err != nil {
		return execution, err
	}

	if err := json.Unmarshal(data, &execution); err != nil {
		return execution, err
	}

	return execution, nil
}

// createComparison creates a comprehensive comparison between two benchmark runs
func createComparison(run1, run2 BenchmarkExecution) ComparisonResult {
	comparison := ComparisonResult{
		Run1: run1,
		Run2: run2,
	}

	// Calculate model deltas
	comparison.ModelDeltas = calculateModelDeltas(run1.Results, run2.Results)

	// Calculate prompt deltas
	comparison.PromptDeltas = calculatePromptDeltas(run1, run2)

	// Calculate hardware differences
	comparison.HardwareDiff = calculateHardwareDiff(run1.Metadata.Hardware, run2.Metadata.Hardware)

	return comparison
}

// calculateModelDeltas compares model performance between two runs
func calculateModelDeltas(run1Results, run2Results []ModelBenchmarkResult) []ModelDelta {
	var deltas []ModelDelta

	// Create maps for quick lookup
	run1Models := make(map[string]ModelBenchmarkResult)
	run2Models := make(map[string]ModelBenchmarkResult)

	for _, result := range run1Results {
		run1Models[result.Model] = result
	}
	for _, result := range run2Results {
		run2Models[result.Model] = result
	}

	// Get all unique models
	allModels := make(map[string]bool)
	for model := range run1Models {
		allModels[model] = true
	}
	for model := range run2Models {
		allModels[model] = true
	}

	// Calculate deltas for each model
	for model := range allModels {
		delta := ModelDelta{
			Model: model,
		}

		run1Result, inRun1 := run1Models[model]
		run2Result, inRun2 := run2Models[model]

		delta.PresentInBoth = inRun1 && inRun2

		if inRun1 {
			delta.Run1TokensPerSec = run1Result.TokensPerSec
			delta.Run1AvgTime = run1Result.AverageTime
		}

		if inRun2 {
			delta.Run2TokensPerSec = run2Result.TokensPerSec
			delta.Run2AvgTime = run2Result.AverageTime
		}

		if delta.PresentInBoth {
			// Calculate deltas and percentage changes
			delta.TokensPerSecDelta = delta.Run2TokensPerSec - delta.Run1TokensPerSec
			if delta.Run1TokensPerSec > 0 {
				delta.TokensPerSecChange = (delta.TokensPerSecDelta / delta.Run1TokensPerSec) * 100
			}

			delta.AvgTimeDelta = delta.Run2AvgTime - delta.Run1AvgTime
			if delta.Run1AvgTime > 0 {
				delta.AvgTimeChange = (delta.AvgTimeDelta / delta.Run1AvgTime) * 100
			}
		}

		deltas = append(deltas, delta)
	}

	// Sort by tokens per second change (descending)
	sort.Slice(deltas, func(i, j int) bool {
		if deltas[i].PresentInBoth && deltas[j].PresentInBoth {
			return deltas[i].TokensPerSecChange > deltas[j].TokensPerSecChange
		}
		return deltas[i].PresentInBoth && !deltas[j].PresentInBoth
	})

	return deltas
}

// calculatePromptDeltas compares prompt performance between two runs
func calculatePromptDeltas(run1, run2 BenchmarkExecution) []PromptDelta {
	var deltas []PromptDelta

	// Get all unique prompts
	allPrompts := make(map[string]string)
	for checksum, text := range run1.Prompts {
		allPrompts[checksum] = text
	}
	for checksum, text := range run2.Prompts {
		allPrompts[checksum] = text
	}

	// For each prompt, compare performance across models
	for checksum, text := range allPrompts {
		delta := PromptDelta{
			PromptChecksum: checksum,
			PromptText:     text,
		}

		// Get performance data for this prompt from both runs
		run1Perf := getPromptPerformanceFromRun(run1.Results, checksum)
		run2Perf := getPromptPerformanceFromRun(run2.Results, checksum)

		// Get all models that tested this prompt
		allModels := make(map[string]bool)
		for model := range run1Perf {
			allModels[model] = true
		}
		for model := range run2Perf {
			allModels[model] = true
		}

		// Calculate deltas for each model on this prompt
		for model := range allModels {
			modelDelta := PromptModelDelta{
				Model: model,
			}

			perf1, inRun1 := run1Perf[model]
			perf2, inRun2 := run2Perf[model]

			modelDelta.PresentInBoth = inRun1 && inRun2

			if inRun1 {
				modelDelta.Run1TokensPerSec = perf1
			}
			if inRun2 {
				modelDelta.Run2TokensPerSec = perf2
			}

			if modelDelta.PresentInBoth {
				modelDelta.TokensPerSecDelta = modelDelta.Run2TokensPerSec - modelDelta.Run1TokensPerSec
				if modelDelta.Run1TokensPerSec > 0 {
					modelDelta.TokensPerSecChange = (modelDelta.TokensPerSecDelta / modelDelta.Run1TokensPerSec) * 100
				}
			}

			delta.ModelPerformances = append(delta.ModelPerformances, modelDelta)
		}

		// Sort model performances by change percentage
		sort.Slice(delta.ModelPerformances, func(i, j int) bool {
			if delta.ModelPerformances[i].PresentInBoth && delta.ModelPerformances[j].PresentInBoth {
				return delta.ModelPerformances[i].TokensPerSecChange > delta.ModelPerformances[j].TokensPerSecChange
			}
			return delta.ModelPerformances[i].PresentInBoth && !delta.ModelPerformances[j].PresentInBoth
		})

		deltas = append(deltas, delta)
	}

	return deltas
}

// getPromptPerformanceFromRun extracts average performance for a prompt across all models in a run
func getPromptPerformanceFromRun(results []ModelBenchmarkResult, promptChecksum string) map[string]float64 {
	performance := make(map[string]float64)

	for _, modelResult := range results {
		var totalTokensPerSec float64
		var count int

		for _, promptResult := range modelResult.PromptResults {
			if promptResult.PromptChecksum == promptChecksum {
				totalTokensPerSec += promptResult.TokensPerSec
				count++
			}
		}

		if count > 0 {
			performance[modelResult.Model] = totalTokensPerSec / float64(count)
		}
	}

	return performance
}

// calculateHardwareDiff compares hardware configurations between two runs
func calculateHardwareDiff(hw1, hw2 HardwareInfo) HardwareDiff {
	diff := HardwareDiff{
		Run1OS:       hw1.OS,
		Run2OS:       hw2.OS,
		Run1Arch:     hw1.Architecture,
		Run2Arch:     hw2.Architecture,
		Run1CPUCount: hw1.CPUCount,
		Run2CPUCount: hw2.CPUCount,
		Run1Cloud:    hw1.Cloud,
		Run2Cloud:    hw2.Cloud,
		Run1GPUs:     hw1.GPUs,
		Run2GPUs:     hw2.GPUs,
	}

	diff.OSChanged = hw1.OS != hw2.OS
	diff.ArchChanged = hw1.Architecture != hw2.Architecture
	diff.CPUCountChanged = hw1.CPUCount != hw2.CPUCount
	diff.CloudChanged = !compareCloudInfo(hw1.Cloud, hw2.Cloud)
	diff.GPUChanged = !compareGPUs(hw1.GPUs, hw2.GPUs)

	return diff
}

// compareCloudInfo compares two CloudInfo structs
func compareCloudInfo(c1, c2 CloudInfo) bool {
	return c1.Provider == c2.Provider && c1.SKU == c2.SKU && c1.Region == c2.Region
}

// compareGPUs compares two GPU slices
func compareGPUs(gpus1, gpus2 []GPUInfo) bool {
	if len(gpus1) != len(gpus2) {
		return false
	}

	for i, gpu1 := range gpus1 {
		if i >= len(gpus2) || gpu1.Name != gpus2[i].Name || gpu1.Memory != gpus2[i].Memory {
			return false
		}
	}

	return true
}

// displayComparison displays the comprehensive comparison results
func displayComparison(comparison ComparisonResult, file1Path, file2Path string) {
	fmt.Printf("🔄 Benchmark Comparison Analysis\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════════\n")
	fmt.Printf("📁 Run 1: %s\n", file1Path)
	fmt.Printf("📁 Run 2: %s\n", file2Path)
	fmt.Printf("🕐 Run 1 Time: %s\n", comparison.Run1.Metadata.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("🕐 Run 2 Time: %s\n\n", comparison.Run2.Metadata.Timestamp.Format("2006-01-02 15:04:05"))

	// Display hardware comparison
	displayHardwareComparison(comparison.HardwareDiff)

	// Display model performance comparison
	displayModelComparison(comparison.ModelDeltas)

	// Display prompt performance comparison (show top 3 most significant)
	displayPromptComparison(comparison.PromptDeltas)
}

// displayHardwareComparison shows hardware differences between runs
func displayHardwareComparison(diff HardwareDiff) {
	fmt.Printf("🖥️  Hardware Environment Comparison\n")
	fmt.Printf("─────────────────────────────────────────────────────────────────\n")

	data := [][]string{
		{"COMPONENT", "RUN 1", "RUN 2", "CHANGED"},
		{"OS", diff.Run1OS, diff.Run2OS, getChangeIcon(diff.OSChanged)},
		{"Architecture", diff.Run1Arch, diff.Run2Arch, getChangeIcon(diff.ArchChanged)},
		{"CPU Count", fmt.Sprintf("%d", diff.Run1CPUCount), fmt.Sprintf("%d", diff.Run2CPUCount), getChangeIcon(diff.CPUCountChanged)},
	}

	// Add cloud info if present
	if diff.Run1Cloud.Provider != "" || diff.Run2Cloud.Provider != "" {
		cloud1 := formatCloudInfo(diff.Run1Cloud)
		cloud2 := formatCloudInfo(diff.Run2Cloud)
		data = append(data, []string{"Cloud", cloud1, cloud2, getChangeIcon(diff.CloudChanged)})
	}

	// Add GPU info if present
	if len(diff.Run1GPUs) > 0 || len(diff.Run2GPUs) > 0 {
		gpu1 := formatGPUInfo(diff.Run1GPUs)
		gpu2 := formatGPUInfo(diff.Run2GPUs)
		data = append(data, []string{"GPUs", gpu1, gpu2, getChangeIcon(diff.GPUChanged)})
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header(data[0])
	table.Bulk(data[1:])
	table.Render()
	fmt.Println()
}

// displayModelComparison shows model performance differences
func displayModelComparison(deltas []ModelDelta) {
	fmt.Printf("🏆 Model Performance Comparison\n")
	fmt.Printf("─────────────────────────────────────────────────────────────────\n")

	var data [][]string
	data = append(data, []string{"MODEL", "RUN 1 (T/S)", "RUN 2 (T/S)", "CHANGE", "CHANGE %", "STATUS"})

	for _, delta := range deltas {
		if !delta.PresentInBoth {
			// Model only in one run
			if delta.Run1TokensPerSec > 0 {
				data = append(data, []string{
					delta.Model, // Remove truncation to show full model name
					fmt.Sprintf("%.1f", delta.Run1TokensPerSec),
					"-",
					"-",
					"-",
					"🔴 Removed",
				})
			} else {
				data = append(data, []string{
					delta.Model, // Remove truncation to show full model name
					"-",
					fmt.Sprintf("%.1f", delta.Run2TokensPerSec),
					"-",
					"-",
					"🟢 Added",
				})
			}
		} else {
			// Model in both runs
			changeIcon := getPerformanceChangeIcon(delta.TokensPerSecChange)
			data = append(data, []string{
				delta.Model, // Remove truncation to show full model name
				fmt.Sprintf("%.1f", delta.Run1TokensPerSec),
				fmt.Sprintf("%.1f", delta.Run2TokensPerSec),
				fmt.Sprintf("%+.1f", delta.TokensPerSecDelta),
				fmt.Sprintf("%+.1f%%", delta.TokensPerSecChange),
				changeIcon,
			})
		}
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header(data[0])
	table.Bulk(data[1:])

	// Configure table to prevent truncation by setting a larger max width
	table.Configure(func(cfg *tablewriter.Config) {
		cfg.MaxWidth = 200 // Set a larger maximum width to prevent truncation
	})

	table.Render()
	fmt.Println()
}

// displayPromptComparison shows prompt-specific performance differences
func displayPromptComparison(deltas []PromptDelta) {
	if len(deltas) == 0 {
		return
	}

	fmt.Printf("🎯 Prompt Performance Comparison (Top 3 Prompts)\n")
	fmt.Printf("─────────────────────────────────────────────────────────────────\n")

	// Show up to 3 prompts with the most significant changes
	maxPrompts := len(deltas)
	if maxPrompts > 3 {
		maxPrompts = 3
	}

	for i := 0; i < maxPrompts; i++ {
		delta := deltas[i]
		fmt.Printf("📝 Prompt %d: %s\n", i+1, truncateString(delta.PromptText, 80))

		var data [][]string
		data = append(data, []string{"MODEL", "RUN 1 (T/S)", "RUN 2 (T/S)", "CHANGE", "CHANGE %"})

		for _, modelPerf := range delta.ModelPerformances {
			if modelPerf.PresentInBoth {
				data = append(data, []string{
					modelPerf.Model, // Remove truncation to show full model name
					fmt.Sprintf("%.1f", modelPerf.Run1TokensPerSec),
					fmt.Sprintf("%.1f", modelPerf.Run2TokensPerSec),
					fmt.Sprintf("%+.1f", modelPerf.TokensPerSecDelta),
					fmt.Sprintf("%+.1f%%", modelPerf.TokensPerSecChange),
				})
			}
		}

		if len(data) > 1 {
			table := tablewriter.NewWriter(os.Stdout)
			table.Header(data[0])
			table.Bulk(data[1:])

			// Configure table to prevent truncation by setting a larger max width
			table.Configure(func(cfg *tablewriter.Config) {
				cfg.MaxWidth = 200 // Set a larger maximum width to prevent truncation
			})

			table.Render()
		}
		fmt.Println()
	}
}

// Helper functions

func getChangeIcon(changed bool) string {
	if changed {
		return "⚠️ Changed"
	}
	return "✅ Same "
}

func getPerformanceChangeIcon(changePercent float64) string {
	if math.Abs(changePercent) < 1.0 {
		return "➖ Minimal"
	} else if changePercent > 5.0 {
		return "📈 Improved"
	} else if changePercent < -5.0 {
		return "📉 Degraded"
	} else if changePercent > 0 {
		return "🟢 Better"
	} else {
		return "🔻 Worse"
	}
}

func formatCloudInfo(cloud CloudInfo) string {
	if cloud.Provider == "" {
		return "None"
	}
	parts := []string{cloud.Provider}
	if cloud.SKU != "" {
		parts = append(parts, cloud.SKU)
	}
	if cloud.Region != "" {
		parts = append(parts, cloud.Region)
	}
	return strings.Join(parts, " / ")
}

func formatGPUInfo(gpus []GPUInfo) string {
	if len(gpus) == 0 {
		return "None"
	}
	var parts []string
	for _, gpu := range gpus {
		part := gpu.Name
		if gpu.Memory != "" {
			part += fmt.Sprintf(" (%s)", gpu.Memory)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func WriteComparison(path string, results []benchmark.BenchmarkResult) error {
	agg := Aggregate(results)
	sort.Slice(agg, func(i, j int) bool {
		return agg[i].TokenPerS > agg[j].TokenPerS
	})

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf(i18n.T("err_file_create"), err)
	}
	defer file.Close()

	fmt.Fprintf(file, "%s\t%s\t%s\t%s\t%s\n",
		i18n.T("header_model"),
		i18n.T("header_time"),
		i18n.T("header_tokens"),
		i18n.T("header_tps"),
		i18n.T("header_rank"),
	)

	for i, r := range agg {
		rank := "-"
		if i == 0 {
			rank = "🥇"
		} else if i == 1 {
			rank = "🥈"
		} else if i == 2 {
			rank = "🥉"
		}
		fmt.Fprintf(file, "%s\t%.2f\t%d\t%.1f\t%s\n", r.Model, r.AvgDuration.Seconds(), r.TotalTokens, r.TokenPerS, rank)
	}

	return nil
}
