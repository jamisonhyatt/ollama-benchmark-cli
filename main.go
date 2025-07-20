package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"ollama-benchmark/internal/benchmark"
	"ollama-benchmark/internal/client"
	"ollama-benchmark/internal/i18n"
	"ollama-benchmark/internal/logs"
	"ollama-benchmark/internal/output"
	"ollama-benchmark/internal/prompt"
)

const baseOllamaUrl string = "http://localhost:11434"

// Config holds all configuration options
type Config struct {
	Language     string
	Mode         string
	APIUrl       string
	Trials       int
	Format       string
	Models       []string // Changed from Model string to Models []string
	TokensOnly   bool
	PromptFile   string
	CompareFile1 string
	CompareFile2 string
	Interactive  bool
}

func main() {
	config := parseFlags()

	// Load language
	if err := i18n.Load(config.Language); err != nil {
		fmt.Printf("⚠️ Language file error: %v\n", err)
		return
	}

	if config.Interactive {
		runInteractiveMode()
	} else {
		runAutomatedMode(config)
	}
}

func parseFlags() Config {
	var config Config
	var modelsFlag string

	// Define flags
	flag.StringVar(&config.Language, "lang", "", "Language (en, tr)")
	flag.StringVar(&config.Mode, "mode", "", "Mode (quick, settings, compare)")
	flag.StringVar(&config.APIUrl, "api-url", baseOllamaUrl, "Ollama API URL")
	flag.IntVar(&config.Trials, "trials", 3, "Number of trials")
	flag.StringVar(&config.Format, "format", "txt", "Output format (txt, csv, json, enhanced-json)")
	flag.StringVar(&modelsFlag, "models", "", "Comma-separated model names or 'all' for all models")
	flag.BoolVar(&config.TokensOnly, "tokens-only", false, "Sort only by tokens per second")
	flag.StringVar(&config.PromptFile, "prompt-file", "", "Path to prompt file (default: built-in prompts)")
	flag.StringVar(&config.CompareFile1, "file1", "", "First file for comparison mode")
	flag.StringVar(&config.CompareFile2, "file2", "", "Second file for comparison mode")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Ollama Benchmark CLI\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [flags]  # Non-interactive mode\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s         # Interactive mode\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s --lang=en --mode=quick --format=enhanced-json\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --lang=en --mode=settings --trials=5 --models=llama3.1:8b,qwen2.5:7b\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --lang=en --mode=settings --models=all\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --lang=en --mode=compare --file1=log1.txt --file2=log2.txt\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Parse models flag
	if modelsFlag != "" {
		if modelsFlag == "all" {
			config.Models = []string{"all"}
		} else {
			// Split comma-separated models and trim whitespace
			models := strings.Split(modelsFlag, ",")
			for _, model := range models {
				trimmed := strings.TrimSpace(model)
				if trimmed != "" {
					config.Models = append(config.Models, trimmed)
				}
			}
		}
	}

	// Check if any flags were provided
	config.Interactive = true
	flag.Visit(func(f *flag.Flag) {
		config.Interactive = false
	})

	// If non-interactive, validate required fields
	if !config.Interactive {
		if config.Language == "" {
			config.Language = "en" // Default language
		}
		if config.Mode == "" {
			fmt.Fprintf(os.Stderr, "Error: --mode is required in non-interactive mode\n")
			flag.Usage()
			os.Exit(1)
		}
		if err := validateConfig(config); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	return config
}

func validateConfig(config Config) error {
	// Validate language
	if config.Language != "en" && config.Language != "tr" {
		return fmt.Errorf("invalid language: %s (supported: en, tr)", config.Language)
	}

	// Validate mode
	if config.Mode != "quick" && config.Mode != "settings" && config.Mode != "compare" {
		return fmt.Errorf("invalid mode: %s (supported: quick, settings, compare)", config.Mode)
	}

	// Validate format
	if config.Format != "txt" && config.Format != "csv" && config.Format != "json" && config.Format != "enhanced-json" {
		return fmt.Errorf("invalid format: %s (supported: txt, csv, json, enhanced-json)", config.Format)
	}

	// Validate compare mode requirements
	if config.Mode == "compare" {
		if config.CompareFile1 == "" || config.CompareFile2 == "" {
			return fmt.Errorf("compare mode requires both --file1 and --file2")
		}
	}

	return nil
}

func runInteractiveMode() {
	fmt.Println("🌍 Language / Dil seçin:")
	fmt.Println("1) English")
	fmt.Println("2) Türkçe")
	fmt.Print("👉 : ")

	var langChoice string
	fmt.Scanln(&langChoice)
	lang := "en"
	if langChoice == "2" {
		lang = "tr"
	}
	if err := i18n.Load(lang); err != nil {
		fmt.Println("⚠️ Language file error:", err)
		return
	}

	fmt.Println(i18n.T("menu_title"))
	fmt.Println(i18n.T("menu_quick"))
	fmt.Println(i18n.T("menu_settings"))
	fmt.Println(i18n.T("menu_compare"))
	fmt.Print(i18n.T("prompt_choose_option") + " ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1":
		runQuickStart(reader)
	case "2":
		runWithSettings(reader)
	case "3":
		runCompareMode(reader)
	default:
		fmt.Println(i18n.T("msg_invalid_choice"))
	}
}

func runAutomatedMode(config Config) {
	switch config.Mode {
	case "quick":
		runAutomatedQuick(config)
	case "settings":
		runAutomatedSettings(config)
	case "compare":
		runAutomatedCompare(config)
	}
}

func runAutomatedQuick(config Config) {
	fmt.Println(i18n.T("quick_starting"))

	prompts, err := prompt.GetDefaultPrompts()
	if err != nil {
		fmt.Printf("Error loading default prompts: %v\n", err)
		return
	}

	models, err := client.GetModelList(config.APIUrl)
	if err != nil {
		fmt.Printf("Error fetching models: %v\n", err)
		return
	}

	allResults := runAllModels(config.APIUrl, models, prompts, 1)
	handleResults(allResults, config.Format, true, config.TokensOnly, config.APIUrl, prompts, 1)
}

func runAutomatedSettings(config Config) {
	var prompts []string
	var err error

	// Get prompts
	if config.PromptFile == "" {
		prompts, err = prompt.GetDefaultPrompts()
		if err != nil {
			fmt.Printf("Error loading default prompts: %v\n", err)
			return
		}
	} else {
		prompts, err = prompt.ReadPromptsFromFile(config.PromptFile)
		if err != nil {
			fmt.Printf("Error reading prompt file: %v\n", err)
			return
		}
	}

	// Get models
	models, err := client.GetModelList(config.APIUrl)
	if err != nil {
		fmt.Printf("Error fetching models: %v\n", err)
		return
	}

	// Run benchmarks
	if len(config.Models) == 0 || (len(config.Models) == 1 && config.Models[0] == "all") {
		// Use all models
		allResults := runAllModels(config.APIUrl, models, prompts, config.Trials)
		handleResults(allResults, config.Format, true, config.TokensOnly, config.APIUrl, prompts, config.Trials)
	} else {
		// Validate all specified models exist first
		for _, model := range config.Models {
			modelExists := false
			for _, m := range models {
				if m == model {
					modelExists = true
					break
				}
			}
			if !modelExists {
				fmt.Printf("Error: Model '%s' not found. Available models:\n", model)
				for _, m := range models {
					fmt.Printf("  - %s\n", m)
				}
				return
			}
		}

		// Run benchmarks for specified models
		var results []benchmark.BenchmarkResult
		for _, model := range config.Models {
			r, err := benchmark.RunBenchmark(config.APIUrl, model, prompts, config.Trials)
			if err != nil {
				fmt.Printf("Error running benchmark for model %s: %v\n", model, err)
				continue
			}
			results = append(results, r...)

			// Unload the model after benchmarking to free memory
			if err := client.UnloadModel(config.APIUrl, model); err != nil {
				fmt.Printf("Warning: Failed to unload model %s: %v\n", model, err)
				// Continue anyway - this is not a critical error
			}
		}

		// Determine if this is multi-model (more than 1 model)
		isMultiModel := len(config.Models) > 1
		handleResults(results, config.Format, isMultiModel, config.TokensOnly, config.APIUrl, prompts, config.Trials)
	}
}

func runAutomatedCompare(config Config) {
	err := logs.CompareLogFiles(config.CompareFile1, config.CompareFile2)
	if err != nil {
		fmt.Printf(i18n.T("msg_compare_error")+"\n", err)
		return
	}
	fmt.Println(i18n.T("msg_compare_complete"))
}

func runAllModels(apiURL string, models []string, prompts []string, trials int) []benchmark.BenchmarkResult {
	var allResults []benchmark.BenchmarkResult
	for _, model := range models {
		results, err := benchmark.RunBenchmark(apiURL, model, prompts, trials)
		if err != nil {
			fmt.Printf(i18n.T("msg_model_error")+"\n", model, err)
			continue
		}
		allResults = append(allResults, results...)

		// Unload the model after benchmarking to free memory
		if err := client.UnloadModel(apiURL, model); err != nil {
			fmt.Printf("Warning: Failed to unload model %s: %v\n", model, err)
			// Continue anyway - this is not a critical error
		}
	}
	return allResults
}

func handleResults(results []benchmark.BenchmarkResult, format string, isMultiModel bool, tokensOnly bool, apiURL string, prompts []string, trials int) {
	if len(results) == 0 {
		fmt.Println(i18n.T("msg_no_results"))
		return
	}

	output.FormatAsTable(results, tokensOnly)

	if !isMultiModel {
		output.PrintDetails(results)
	}

	// Show comparison for multi-model runs (regardless of output format)
	if isMultiModel {
		output.ShowComparison(results)
	}

	if format == "enhanced-json" {
		// For enhanced-json, only create the enhanced JSON file
		config := output.BenchmarkConfig{
			APIEndpoint: apiURL,
			Trials:      trials,
			Prompts:     prompts,
		}
		summaryFile := fmt.Sprintf("benchmark_summary_result_%s.json", timestamp())
		output.WriteEnhancedJSON(summaryFile, results, config)
		fmt.Printf("✅ "+i18n.T("msg_benchmark_complete")+" Enhanced JSON: %s\n", summaryFile)
	} else {
		// For other formats, create all the output files
		modelGroups := output.GroupByModel(results)
		for model, group := range modelGroups {
			filename := fmt.Sprintf("benchmark_detail_%s_%s.txt", sanitizeFilename(model), timestamp())
			output.WriteTXT(filename, group, false)
		}

		summaryFile := fmt.Sprintf("benchmark_summary_result_%s.%s", timestamp(), format)
		switch format {
		case "csv":
			output.WriteCSV(summaryFile, results)
		case "json":
			output.WriteJSON(summaryFile, results)
		case "txt":
			output.WriteTXT(summaryFile, results, tokensOnly)
		}

		if len(modelGroups) > 1 {
			compFile := fmt.Sprintf("benchmark_summary_comparison_%s.txt", timestamp())
			output.WriteComparison(compFile, results)
		}

		fmt.Println("✅ " + i18n.T("msg_benchmark_complete") + " Log: benchmark.log")
	}

	logs.AppendPerformanceLog(results)
}

func timestamp() string {
	return time.Now().Format("20060102-150405")
}

func sanitizeFilename(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, ":", "_"), "/", "_")
}

func runQuickStart(reader *bufio.Reader) {
	fmt.Print(i18n.T("prompt_api_url") + " (default: " + baseOllamaUrl + "): ")
	apiURL, _ := reader.ReadString('\n')
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		apiURL = baseOllamaUrl
	}

	fmt.Println(i18n.T("quick_prompt_path"))
	fmt.Println(i18n.T("quick_trials"))
	fmt.Println(i18n.T("quick_mode"))
	fmt.Println(i18n.T("quick_output"))
	fmt.Println(i18n.T("quick_log"))
	fmt.Println(i18n.T("quick_starting"))

	prompts, _ := prompt.GetDefaultPrompts()
	models, _ := client.GetModelList(apiURL)
	allResults := runAllModels(apiURL, models, prompts, 1)

	handleResults(allResults, "txt", true, false, apiURL, prompts, 1)
}

func runWithSettings(reader *bufio.Reader) {
	fmt.Print(i18n.T("prompt_api_url") + " (default: " + baseOllamaUrl + "): ")
	apiURL, _ := reader.ReadString('\n')
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		apiURL = baseOllamaUrl
	}

	fmt.Print(i18n.T("prompt_prompt_file") + " (default: built-in prompts): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var prompts []string
	var err error

	if input == "" {
		// Use embedded default prompts
		prompts, err = prompt.GetDefaultPrompts()
		if err != nil {
			fmt.Printf("Error loading default prompts: %v\n", err)
			return
		}
	} else {
		// Use custom file
		prompts, err = prompt.ReadPromptsFromFile(input)
		if err != nil {
			fmt.Printf("Error reading prompt file: %v\n", err)
			return
		}
	}

	fmt.Print(i18n.T("prompt_trials") + " (default: 3): ")
	trialStr, _ := reader.ReadString('\n')
	trialStr = strings.TrimSpace(trialStr)
	trials := 3
	if trialStr != "" {
		if t, err := strconv.Atoi(trialStr); err == nil {
			trials = t
		}
	}

	fmt.Println(i18n.T("prompt_output_format"))
	fmt.Println("1) TXT")
	fmt.Println("2) CSV")
	fmt.Println("3) JSON")
	fmt.Println("4) Enhanced JSON")
	fmt.Print("👉 Choose format (default: 1): ")
	formatChoice, _ := reader.ReadString('\n')
	formatChoice = strings.TrimSpace(formatChoice)
	if formatChoice == "" {
		formatChoice = "1"
	}

	var format string
	switch formatChoice {
	case "1":
		format = "txt"
	case "2":
		format = "csv"
	case "3":
		format = "json"
	case "4":
		format = "enhanced-json"
	default:
		fmt.Println(i18n.T("msg_invalid_format"))
		return
	}

	fmt.Print(i18n.T("prompt_tokens_only") + " ")
	tokensOnlyStr, _ := reader.ReadString('\n')
	tokensOnly := strings.TrimSpace(strings.ToLower(tokensOnlyStr)) == "e" || tokensOnlyStr == "y"

	fmt.Println(i18n.T("msg_loading_models"))
	models, _ := client.GetModelList(apiURL)

	for i, m := range models {
		fmt.Printf("%d) %s\n", i+1, m)
	}
	if len(models) > 1 {
		fmt.Printf("%d) %s\n", len(models)+1, i18n.T("prompt_all_models"))
	}

	fmt.Print(i18n.T("prompt_model_selection") + ": ")
	selection, _ := reader.ReadString('\n')
	selection = strings.TrimSpace(selection)
	choice, _ := strconv.Atoi(selection)

	if choice > 0 && choice <= len(models) {
		model := models[choice-1]
		results, _ := benchmark.RunBenchmark(apiURL, model, prompts, trials)
		handleResults(results, format, false, tokensOnly, apiURL, prompts, trials)
	} else if choice == len(models)+1 {
		allResults := runAllModels(apiURL, models, prompts, trials)
		handleResults(allResults, format, true, tokensOnly, apiURL, prompts, trials)
	} else {
		fmt.Println(i18n.T("msg_invalid_choice"))
	}
}

func runCompareMode(reader *bufio.Reader) {
	fmt.Print(i18n.T("prompt_compare_file1") + ": ")
	file1, _ := reader.ReadString('\n')
	file1 = strings.TrimSpace(file1)

	fmt.Print(i18n.T("prompt_compare_file2") + ": ")
	file2, _ := reader.ReadString('\n')
	file2 = strings.TrimSpace(file2)

	if file1 == "" || file2 == "" {
		fmt.Println(i18n.T("msg_invalid_files"))
		return
	}

	err := logs.CompareLogFiles(file1, file2)
	if err != nil {
		fmt.Printf(i18n.T("msg_compare_error")+"\n", err)
		return
	}

	fmt.Println(i18n.T("msg_compare_complete"))
}
