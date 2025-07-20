package output

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"ollama-benchmark/internal/benchmark"
	"github.com/olekukonko/tablewriter"
)

// Enhanced data structures for comprehensive analysis
type BenchmarkExecution struct {
	Metadata ExecutionMetadata      `json:"metadata"`
	Prompts  map[string]string      `json:"prompts"`
	Results  []ModelBenchmarkResult `json:"results"`
}

type ExecutionMetadata struct {
	Timestamp     time.Time       `json:"timestamp"`
	Version       string          `json:"version"`
	Hardware      HardwareInfo    `json:"hardware"`
	Configuration BenchmarkConfig `json:"configuration"`
	Duration      time.Duration   `json:"total_duration"`
}

type HardwareInfo struct {
	OS           string    `json:"os"`
	Architecture string    `json:"architecture"`
	CPUCount     int       `json:"cpu_count"`
	Hostname     string    `json:"hostname,omitempty"`
	GPUs         []GPUInfo `json:"gpus,omitempty"`
	Cloud        CloudInfo `json:"cloud,omitempty"`
}

type GPUInfo struct {
	Name   string `json:"name"`
	Memory string `json:"memory,omitempty"`
	Driver string `json:"driver,omitempty"`
}

type CloudInfo struct {
	Provider string `json:"provider,omitempty"`
	SKU      string `json:"sku,omitempty"`
	Region   string `json:"region,omitempty"`
}

type BenchmarkConfig struct {
	APIEndpoint string   `json:"api_endpoint"`
	Trials      int      `json:"trials"`
	Prompts     []string `json:"prompts"`
}

type ModelBenchmarkResult struct {
	Model         string          `json:"model"`
	TotalTime     float64         `json:"total_time_seconds"`
	AverageTime   float64         `json:"average_time_seconds"`
	TotalTokens   int             `json:"total_tokens"`
	TokensPerSec  float64         `json:"tokens_per_second"`
	PromptResults []PromptResult  `json:"prompt_results"`
	Aggregated    AggregatedStats `json:"aggregated_stats"`
}

type PromptResult struct {
	PromptChecksum string  `json:"prompt_checksum"`
	Trial          int     `json:"trial"`
	Duration       float64 `json:"duration_seconds"`
	Tokens         int     `json:"tokens"`
	TokensPerSec   float64 `json:"tokens_per_second"`
}

type AggregatedStats struct {
	MinTime   float64 `json:"min_time_seconds"`
	MaxTime   float64 `json:"max_time_seconds"`
	StdDev    float64 `json:"std_dev_seconds"`
	MinTokens int     `json:"min_tokens"`
	MaxTokens int     `json:"max_tokens"`
}

// WriteEnhancedJSON writes a comprehensive, machine-readable JSON format
func WriteEnhancedJSON(path string, results []benchmark.BenchmarkResult, config BenchmarkConfig) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	execution := createBenchmarkExecution(results, config)

	data, err := json.MarshalIndent(execution, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Helper function to create comprehensive benchmark execution data
func createBenchmarkExecution(results []benchmark.BenchmarkResult, config BenchmarkConfig) BenchmarkExecution {
	hostname, _ := os.Hostname()

	execution := BenchmarkExecution{
		Metadata: ExecutionMetadata{
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Hardware: HardwareInfo{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH,
				CPUCount:     runtime.NumCPU(),
				Hostname:     hostname,
				GPUs:         detectGPUs(),
				Cloud:        detectCloudInfo(),
			},
			Configuration: config,
		},
		Prompts: make(map[string]string),
		Results: []ModelBenchmarkResult{},
	}

	// Calculate total execution duration
	if len(results) > 0 {
		for _, r := range results {
			execution.Metadata.Duration += r.Duration
		}
	}

	// Group results by model for comprehensive analysis
	modelGroups := GroupByModel(results)

	for model, modelResults := range modelGroups {
		modelResult := ModelBenchmarkResult{
			Model:         model,
			PromptResults: []PromptResult{},
		}

		var totalTime time.Duration
		var totalTokens int
		var times []float64
		var tokenCounts []int

		// Process individual prompt results
		for _, r := range modelResults {
			// Calculate prompt checksum
			promptChecksum := calculateChecksum(r.Prompt)

			// Add prompt to prompts map if not already present
			execution.Prompts[promptChecksum] = r.Prompt

			modelResult.PromptResults = append(modelResult.PromptResults, PromptResult{
				PromptChecksum: promptChecksum,
				Trial:          r.Trial,
				Duration:       r.Duration.Seconds(),
				Tokens:         r.Tokens,
				TokensPerSec:   r.TokenPerS,
			})

			totalTime += r.Duration
			totalTokens += r.Tokens
			times = append(times, r.Duration.Seconds())
			tokenCounts = append(tokenCounts, r.Tokens)
		}

		// Calculate aggregated statistics
		modelResult.TotalTime = totalTime.Seconds()
		modelResult.AverageTime = totalTime.Seconds() / float64(len(modelResults))
		modelResult.TotalTokens = totalTokens
		modelResult.TokensPerSec = float64(totalTokens) / totalTime.Seconds()

		// Calculate statistical measures
		modelResult.Aggregated = calculateStats(times, tokenCounts)

		execution.Results = append(execution.Results, modelResult)
	}

	return execution
}

// Calculate statistical measures for better analysis
func calculateStats(times []float64, tokens []int) AggregatedStats {
	if len(times) == 0 {
		return AggregatedStats{}
	}

	// Time statistics
	minTime, maxTime := times[0], times[0]
	var timeSum float64
	for _, t := range times {
		if t < minTime {
			minTime = t
		}
		if t > maxTime {
			maxTime = t
		}
		timeSum += t
	}

	// Calculate standard deviation
	mean := timeSum / float64(len(times))
	variance := 0.0
	for _, t := range times {
		variance += (t - mean) * (t - mean)
	}
	stdDev := 0.0
	if len(times) > 1 {
		stdDev = math.Sqrt(variance / float64(len(times)-1))
	}

	// Token statistics
	minTokens, maxTokens := tokens[0], tokens[0]
	for _, t := range tokens {
		if t < minTokens {
			minTokens = t
		}
		if t > maxTokens {
			maxTokens = t
		}
	}

	return AggregatedStats{
		MinTime:   minTime,
		MaxTime:   maxTime,
		StdDev:    stdDev,
		MinTokens: minTokens,
		MaxTokens: maxTokens,
	}
}

// detectGPUs attempts to detect GPU information across different platforms
func detectGPUs() []GPUInfo {
	var gpus []GPUInfo

	switch runtime.GOOS {
	case "darwin": // macOS
		gpus = append(gpus, detectMacGPUs()...)
	case "linux":
		gpus = append(gpus, detectLinuxGPUs()...)
	case "windows":
		gpus = append(gpus, detectWindowsGPUs()...)
	}

	return gpus
}

// detectMacGPUs detects GPU information on macOS using system_profiler
func detectMacGPUs() []GPUInfo {
	var gpus []GPUInfo

	cmd := exec.Command("system_profiler", "SPDisplaysDataType", "-json")
	output, err := cmd.Output()
	if err != nil {
		return gpus
	}

	// Parse the JSON output to extract GPU information
	var data map[string]interface{}
	if err := json.Unmarshal(output, &data); err != nil {
		return gpus
	}

	if displays, ok := data["SPDisplaysDataType"].([]interface{}); ok {
		for _, display := range displays {
			if displayMap, ok := display.(map[string]interface{}); ok {
				gpu := GPUInfo{}
				if name, ok := displayMap["sppci_model"].(string); ok {
					gpu.Name = name
				}
				if memory, ok := displayMap["spdisplays_vram"].(string); ok {
					gpu.Memory = memory
				}
				if gpu.Name != "" {
					gpus = append(gpus, gpu)
				}
			}
		}
	}

	return gpus
}

// detectLinuxGPUs detects GPU information on Linux using various methods
func detectLinuxGPUs() []GPUInfo {
	var gpus []GPUInfo

	// Try nvidia-smi first for NVIDIA GPUs
	if nvidiaGPUs := detectNvidiaGPUs(); len(nvidiaGPUs) > 0 {
		gpus = append(gpus, nvidiaGPUs...)
	}

	// Try lspci for other GPUs
	if lspciGPUs := detectLspciGPUs(); len(lspciGPUs) > 0 {
		gpus = append(gpus, lspciGPUs...)
	}

	return gpus
}

// detectNvidiaGPUs uses nvidia-smi to detect NVIDIA GPUs
func detectNvidiaGPUs() []GPUInfo {
	var gpus []GPUInfo

	cmd := exec.Command("nvidia-smi", "--query-gpu=name,memory.total,driver_version", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return gpus
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ", ")
		if len(parts) >= 3 {
			gpu := GPUInfo{
				Name:   strings.TrimSpace(parts[0]),
				Memory: fmt.Sprintf("%s MB", strings.TrimSpace(parts[1])),
				Driver: strings.TrimSpace(parts[2]),
			}
			gpus = append(gpus, gpu)
		}
	}

	return gpus
}

// detectLspciGPUs uses lspci to detect GPUs
func detectLspciGPUs() []GPUInfo {
	var gpus []GPUInfo

	cmd := exec.Command("lspci", "-v")
	output, err := cmd.Output()
	if err != nil {
		return gpus
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "vga compatible controller") ||
			strings.Contains(strings.ToLower(line), "3d controller") {
			parts := strings.Split(line, ": ")
			if len(parts) >= 2 {
				gpu := GPUInfo{
					Name: strings.TrimSpace(parts[1]),
				}
				gpus = append(gpus, gpu)
			}
		}
	}

	return gpus
}

// detectWindowsGPUs detects GPU information on Windows using wmic
func detectWindowsGPUs() []GPUInfo {
	var gpus []GPUInfo

	cmd := exec.Command("wmic", "path", "win32_VideoController", "get", "name,AdapterRAM", "/format:csv")
	output, err := cmd.Output()
	if err != nil {
		return gpus
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header and empty lines
		}
		parts := strings.Split(line, ",")
		if len(parts) >= 3 {
			name := strings.TrimSpace(parts[2])
			memory := strings.TrimSpace(parts[1])
			if name != "" && name != "Name" {
				gpu := GPUInfo{
					Name: name,
				}
				if memory != "" && memory != "AdapterRAM" {
					gpu.Memory = fmt.Sprintf("%s bytes", memory)
				}
				gpus = append(gpus, gpu)
			}
		}
	}

	return gpus
}

// detectCloudInfo attempts to detect cloud provider and instance information
func detectCloudInfo() CloudInfo {
	cloud := CloudInfo{}

	// Try Azure first
	if azureInfo := detectAzureInfo(); azureInfo.Provider != "" {
		return azureInfo
	}

	// Try AWS
	if awsInfo := detectAWSInfo(); awsInfo.Provider != "" {
		return awsInfo
	}

	// Try GCP
	if gcpInfo := detectGCPInfo(); gcpInfo.Provider != "" {
		return gcpInfo
	}

	return cloud
}

// detectAzureInfo detects Azure instance metadata
func detectAzureInfo() CloudInfo {
	cloud := CloudInfo{}

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("GET", "http://169.254.169.254/metadata/instance?api-version=2021-02-01", nil)
	if err != nil {
		return cloud
	}
	req.Header.Set("Metadata", "true")

	resp, err := client.Do(req)
	if err != nil {
		return cloud
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return cloud
	}

	var metadata map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return cloud
	}

	cloud.Provider = "Azure"

	if compute, ok := metadata["compute"].(map[string]interface{}); ok {
		if vmSize, ok := compute["vmSize"].(string); ok {
			cloud.SKU = vmSize
		}
		if location, ok := compute["location"].(string); ok {
			cloud.Region = location
		}
	}

	return cloud
}

// detectAWSInfo detects AWS instance metadata
func detectAWSInfo() CloudInfo {
	cloud := CloudInfo{}

	client := &http.Client{Timeout: 2 * time.Second}

	// Get instance type
	resp, err := client.Get("http://169.254.169.254/latest/meta-data/instance-type")
	if err != nil {
		return cloud
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return cloud
	}

	cloud.Provider = "AWS"

	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	if n > 0 {
		cloud.SKU = string(buf[:n])
	}

	// Get region
	resp2, err := client.Get("http://169.254.169.254/latest/meta-data/placement/availability-zone")
	if err == nil {
		defer resp2.Body.Close()
		if resp2.StatusCode == 200 {
			buf2 := make([]byte, 256)
			n2, _ := resp2.Body.Read(buf2)
			if n2 > 0 {
				az := string(buf2[:n2])
				// Remove the last character to get region (e.g., us-west-2a -> us-west-2)
				if len(az) > 0 {
					cloud.Region = az[:len(az)-1]
				}
			}
		}
	}

	return cloud
}

// detectGCPInfo detects Google Cloud Platform instance metadata
func detectGCPInfo() CloudInfo {
	cloud := CloudInfo{}

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/machine-type", nil)
	if err != nil {
		return cloud
	}
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := client.Do(req)
	if err != nil {
		return cloud
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return cloud
	}

	cloud.Provider = "GCP"

	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	if n > 0 {
		machineType := string(buf[:n])
		// Extract just the machine type name from the full path
		parts := strings.Split(machineType, "/")
		if len(parts) > 0 {
			cloud.SKU = parts[len(parts)-1]
		}
	}

	// Get zone
	req2, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/zone", nil)
	if err == nil {
		req2.Header.Set("Metadata-Flavor", "Google")
		resp2, err := client.Do(req2)
		if err == nil {
			defer resp2.Body.Close()
			if resp2.StatusCode == 200 {
				buf2 := make([]byte, 256)
				n2, _ := resp2.Body.Read(buf2)
				if n2 > 0 {
					zone := string(buf2[:n2])
					// Extract zone from full path
					parts := strings.Split(zone, "/")
					if len(parts) > 0 {
						cloud.Region = parts[len(parts)-1]
					}
				}
			}
		}
	}

	return cloud
}

// calculateChecksum calculates the SHA-256 checksum of a given string
func calculateChecksum(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

// EvaluateSummaryFile reads an enhanced-json summary file and displays performance analysis metrics
func EvaluateSummaryFile(filePath string) error {
	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the JSON
	var execution BenchmarkExecution
	if err := json.Unmarshal(data, &execution); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Display summary information
	fmt.Printf("📊 Benchmark Summary Analysis\n")
	fmt.Printf("═══════════════════════════════════════\n")
	fmt.Printf("🕐 Execution Time: %s\n", execution.Metadata.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("⚙️  Configuration: %d trials, %d prompts\n", execution.Metadata.Configuration.Trials, len(execution.Metadata.Configuration.Prompts))
	fmt.Printf("🖥️  Hardware: %s/%s (%d CPUs)\n", execution.Metadata.Hardware.OS, execution.Metadata.Hardware.Architecture, execution.Metadata.Hardware.CPUCount)

	if len(execution.Metadata.Hardware.GPUs) > 0 {
		fmt.Printf("🎮 GPUs:\n")
		for _, gpu := range execution.Metadata.Hardware.GPUs {
			fmt.Printf("   - %s", gpu.Name)
			if gpu.Memory != "" {
				fmt.Printf(" (%s)", gpu.Memory)
			}
			fmt.Println()
		}
	} else {
		fmt.Println("🎮 No GPUs detected")
	}

	if execution.Metadata.Hardware.Cloud.Provider != "" {
		fmt.Printf("☁️  Cloud: %s", execution.Metadata.Hardware.Cloud.Provider)
		if execution.Metadata.Hardware.Cloud.SKU != "" {
			fmt.Printf(" (%s)", execution.Metadata.Hardware.Cloud.SKU)
		}
		if execution.Metadata.Hardware.Cloud.Region != "" {
			fmt.Printf(" in %s", execution.Metadata.Hardware.Cloud.Region)
		}
		fmt.Println()
	}

	fmt.Printf("⏱️  Total Duration: %s\n\n", execution.Metadata.Duration)

	// Display model performance comparison
	if len(execution.Results) > 1 {
		displayModelComparisonWithPrompts(execution.Results, execution.Prompts)
	} else if len(execution.Results) == 1 {
		displaySingleModelAnalysis(execution.Results[0], execution.Prompts)
	} else {
		fmt.Println("⚠️  No results found in the summary file")
	}

	return nil
}

// displayModelComparisonWithPrompts shows a comparison table between multiple models using tablewriter
func displayModelComparisonWithPrompts(results []ModelBenchmarkResult, prompts map[string]string) {
	fmt.Printf("🏆 Model Performance Comparison\n")

	// Sort by tokens per second (descending)
	sortedResults := make([]ModelBenchmarkResult, len(results))
	copy(sortedResults, results)
	sort.Slice(sortedResults, func(i, j int) bool {
		return sortedResults[i].TokensPerSec > sortedResults[j].TokensPerSec
	})

	// Prepare data for the table
	var data [][]string
	data = append(data, []string{"MODEL", "TOTAL TIME (S)", "AVG TIME (S)", "TOKENS", "TOKENS/SEC"})

	// Add data rows
	for _, result := range sortedResults {
		data = append(data, []string{
			truncateString(result.Model, 35),
			fmt.Sprintf("%.2f", result.TotalTime),
			fmt.Sprintf("%.2f", result.AverageTime),
			fmt.Sprintf("%d", result.TotalTokens),
			fmt.Sprintf("%.1f", result.TokensPerSec),
		})
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header(data[0])
	table.Bulk(data[1:])
	table.Render()
	fmt.Println()

	// Display performance statistics
	if len(results) > 1 {
		fastest := sortedResults[0]
		slowest := sortedResults[len(sortedResults)-1]

		fmt.Printf("📈 Performance Insights:\n")
		fmt.Printf("   🥇 Fastest: %s (%.1f tokens/sec)\n", fastest.Model, fastest.TokensPerSec)
		fmt.Printf("   🐌 Slowest: %s (%.1f tokens/sec)\n", slowest.Model, slowest.TokensPerSec)

		if slowest.TokensPerSec > 0 {
			speedup := fastest.TokensPerSec / slowest.TokensPerSec
			fmt.Printf("   ⚡ Speed difference: %.1fx faster\n", speedup)
		}

		// Calculate average performance
		var totalTokensPerSec float64
		for _, result := range results {
			totalTokensPerSec += result.TokensPerSec
		}
		avgTokensPerSec := totalTokensPerSec / float64(len(results))
		fmt.Printf("   📊 Average performance: %.1f tokens/sec\n\n", avgTokensPerSec)
	}

	// Display prompt performance summaries with prompts map
	displayPromptPerformanceSummaryWithPrompts(results, prompts)
}

// displayPromptPerformanceSummaryWithPrompts shows performance tables for each prompt
func displayPromptPerformanceSummaryWithPrompts(results []ModelBenchmarkResult, prompts map[string]string) {
	// First, gather all prompts and their results across all models
	promptResults := make(map[string][]PromptPerformance)

	// Extract prompt results from all models
	for _, modelResult := range results {
		for _, promptResult := range modelResult.PromptResults {
			promptResults[promptResult.PromptChecksum] = append(promptResults[promptResult.PromptChecksum], PromptPerformance{
				Model:        modelResult.Model,
				TokensPerSec: promptResult.TokensPerSec,
				Duration:     promptResult.Duration,
				Tokens:       promptResult.Tokens,
				Trial:        promptResult.Trial,
			})
		}
	}

	if len(promptResults) == 0 {
		return
	}

	fmt.Printf("🎯 Prompt Performance Analysis\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════════\n\n")

	// Display a table for each prompt
	for promptChecksum, performances := range promptResults {
		// Find the prompt text from the prompts map
		promptText := prompts[promptChecksum]
		if promptText == "" {
			promptText = fmt.Sprintf("Prompt %s", promptChecksum[:8]) // Show first 8 chars of checksum
		}

		fmt.Printf("📝 Prompt: %s\n", truncateString(promptText, 80))
		fmt.Printf("─────────────────────────────────────────────────────────────────\n")

		// Sort by tokens per second (descending)
		sort.Slice(performances, func(i, j int) bool {
			return performances[i].TokensPerSec > performances[j].TokensPerSec
		})

		// Prepare data for the table
		var data [][]string
		data = append(data, []string{"RANK", "MODEL", "TOKENS/SEC", "TIME (S)", "TOKENS", "TRIAL"})

		// Show top performers (limit to top 5 if there are many)
		maxShow := len(performances)
		if maxShow > 5 {
			maxShow = 5
		}

		for i := 0; i < maxShow; i++ {
			perf := performances[i]
			rank := ""
			switch i {
			case 0:
				rank = "🥇"
			case 1:
				rank = "🥈"
			case 2:
				rank = "🥉"
			default:
				rank = fmt.Sprintf("%d", i+1)
			}

			data = append(data, []string{
				rank,
				truncateString(perf.Model, 30),
				fmt.Sprintf("%.1f", perf.TokensPerSec),
				fmt.Sprintf("%.2f", perf.Duration),
				fmt.Sprintf("%d", perf.Tokens),
				fmt.Sprintf("%d", perf.Trial),
			})
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader(data[0])
		table.SetBorder(true)
		table.SetCenterSeparator("│")
		table.SetColumnSeparator("│")
		table.SetRowSeparator("─")
		table.SetHeaderAlignment(tablewriter.ALIGN_CENTER)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.AppendBulk(data[1:])
		table.Render()
		fmt.Println()
	}
}

// displaySingleModelAnalysis shows detailed analysis for a single model
func displaySingleModelAnalysis(result ModelBenchmarkResult, prompts map[string]string) {
	fmt.Printf("🔍 Single Model Analysis: %s\n", result.Model)
	fmt.Printf("═══════════════════════════════════════════════════════════════════\n")
	fmt.Printf("📊 Overall Performance:\n")
	fmt.Printf("   Total Time: %.2f seconds\n", result.TotalTime)
	fmt.Printf("   Average Time: %.2f seconds\n", result.AverageTime)
	fmt.Printf("   Total Tokens: %d\n", result.TotalTokens)
	fmt.Printf("   Tokens/Second: %.1f\n\n", result.TokensPerSec)

	fmt.Printf("📈 Statistical Analysis:\n")
	fmt.Printf("   Min Time: %.2f seconds\n", result.Aggregated.MinTime)
	fmt.Printf("   Max Time: %.2f seconds\n", result.Aggregated.MaxTime)
	fmt.Printf("   Std Deviation: %.2f seconds\n", result.Aggregated.StdDev)
	fmt.Printf("   Min Tokens: %d\n", result.Aggregated.MinTokens)
	fmt.Printf("   Max Tokens: %d\n\n", result.Aggregated.MaxTokens)

	if len(result.PromptResults) > 0 {
		fmt.Printf("🔸 Detailed Results by Prompt:\n")
		for _, promptResult := range result.PromptResults {
			// Find the prompt text using checksum
			promptText := "Unknown prompt"
			for checksum, text := range prompts {
				if checksum == promptResult.PromptChecksum {
					promptText = truncateString(text, 60)
					break
				}
			}

			fmt.Printf("   Trial %d | %s\n", promptResult.Trial, promptText)
			fmt.Printf("     ➜ Tokens: %d | Time: %.2fs | Tokens/sec: %.1f\n\n",
				promptResult.Tokens, promptResult.Duration, promptResult.TokensPerSec)
		}
	}
}

// PromptPerformance holds performance data for a specific prompt and model
type PromptPerformance struct {
	Model        string
	TokensPerSec float64
	Duration     float64
	Tokens       int
	Trial        int
}

// truncateString truncates a string to a maximum length and adds "..." if necessary
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
