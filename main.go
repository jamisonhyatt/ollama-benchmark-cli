package main

import (
	"bufio"
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

func main() {
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

	fmt.Print(i18n.T("prompt_output_format") + " (default: txt, options: csv/json/txt/enhanced-json): ")
	format, _ := reader.ReadString('\n')
	format = strings.TrimSpace(format)
	if format == "" {
		format = "txt"
	}
	if format != "csv" && format != "json" && format != "txt" && format != "enhanced-json" {
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

func runAllModels(apiURL string, models []string, prompts []string, trials int) []benchmark.BenchmarkResult {
	var allResults []benchmark.BenchmarkResult
	for _, model := range models {
		results, err := benchmark.RunBenchmark(apiURL, model, prompts, trials)
		if err != nil {
			fmt.Printf(i18n.T("msg_model_error")+"\n", model, err)
			continue
		}
		allResults = append(allResults, results...)
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
	case "enhanced-json":
		config := output.BenchmarkConfig{
			APIEndpoint: apiURL,
			Trials:      trials,
			Prompts:     prompts,
		}
		output.WriteEnhancedJSON(summaryFile, results, config)
	}

	if len(modelGroups) > 1 {
		compFile := fmt.Sprintf("benchmark_summary_comparison_%s.txt", timestamp())
		output.ShowComparison(results)
		output.WriteComparison(compFile, results)
	}

	logs.AppendPerformanceLog(results)

	fmt.Println(i18n.T("msg_benchmark_complete"))
}

func timestamp() string {
	return time.Now().Format("20060102-150405")
}

func sanitizeFilename(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, ":", "_"), "/", "_")
}
