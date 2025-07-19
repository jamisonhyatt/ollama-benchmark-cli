package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ollama-benchmark/internal/i18n"
)

type BenchmarkResult struct {
	Model     string
	Prompt    string
	Trial     int
	Tokens    int
	Duration  time.Duration
	TokenPerS float64
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	EvalCount int    `json:"eval_count"`
}

func RunBenchmark(apiURL, model string, prompts []string, trials int) ([]BenchmarkResult, error) {
	// Start the elapsed time display
	startTime := time.Now()
	stopTimer := make(chan bool)

	// Start a goroutine to continuously update the elapsed time
	go func() {
		ticker := time.NewTicker(time.Second) // Update every 500ms
		defer ticker.Stop()

		for {
			select {
			case <-stopTimer:
				return
			case <-ticker.C:
				elapsed := time.Since(startTime)
				fmt.Printf("\r⏳ Running benchmark for '%s' with %d prompt(s) x %d trial(s)... %s",
					model, len(prompts), trials, formatDuration(elapsed))
			}
		}
	}()

	// Warmup: ensure model is loaded
	_, err := sendPrompt(apiURL, model, "Hello")
	if err != nil {
		stopTimer <- true
		fmt.Print("\n") // Move to next line after stopping timer
		return nil, fmt.Errorf("warmup failed: %w", err)
	}

	// Small delay to ensure model is fully loaded
	time.Sleep(1 * time.Second)

	var results []BenchmarkResult

	for _, prompt := range prompts {
		for i := 0; i < trials; i++ {
			start := time.Now()

			tokenCount, err := sendPrompt(apiURL, model, prompt)
			if err != nil {
				stopTimer <- true
				fmt.Print("\n") // Move to next line after stopping timer
				return nil, fmt.Errorf(i18n.T("err_prompt_send"), model, err)
			}

			duration := time.Since(start)
			tokensPerSec := float64(tokenCount) / duration.Seconds()

			results = append(results, BenchmarkResult{
				Model:     model,
				Prompt:    prompt,
				Trial:     i + 1,
				Tokens:    tokenCount,
				Duration:  duration,
				TokenPerS: tokensPerSec,
			})
		}
	}

	// Stop the timer and move to next line
	stopTimer <- true
	fmt.Print("\n")

	return results, nil
}

func sendPrompt(apiURL, model, prompt string) (int, error) {
	body := ollamaRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}
	data, _ := json.Marshal(body)

	resp, err := http.Post(apiURL+"/api/generate", "application/json", bytes.NewReader(data))
	if err != nil {
		return 0, fmt.Errorf(i18n.T("err_ollama_connection"), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf(i18n.T("err_ollama_status"), resp.Status)
	}

	var res ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, fmt.Errorf(i18n.T("err_ollama_parse"), err)
	}

	tokenCount := res.EvalCount
	if tokenCount == 0 {
		tokenCount = len(res.Response) / 4
	}

	return tokenCount, nil
}

// Helper function to format duration in a nice way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}
