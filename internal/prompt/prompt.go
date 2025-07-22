package prompt

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"ollama-benchmark/internal/i18n"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed default_prompts.txt
var defaultPromptsData string

// PromptFile represents the structure of a YAML prompt file
type PromptFile struct {
	Prompts []PromptEntry `yaml:"prompts"`
}

// PromptEntry represents a single prompt in the YAML file
type PromptEntry struct {
	Prompt string `yaml:"prompt"`
}

// JSONLPrompt represents a single line in JSONL format
type JSONLPrompt struct {
	Prompt string `json:"prompt"`
}

// GetDefaultPrompts returns the embedded default prompts
func GetDefaultPrompts() ([]string, error) {
	var prompts []string
	lines := strings.Split(strings.TrimSpace(defaultPromptsData), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			prompts = append(prompts, line)
		}
	}

	if len(prompts) == 0 {
		return nil, fmt.Errorf(i18n.T("err_file_empty"))
	}

	return prompts, nil
}

// ReadPromptsFromFile reads prompts from an external file (for custom usage)
// Supports .txt (line-by-line), .yaml/.yml (YAML format), and .jsonl (JSONL format)
func ReadPromptsFromFile(filePath string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".yaml", ".yml":
		return readYAMLPrompts(filePath)
	case ".jsonl":
		return readJSONLPrompts(filePath)
	default:
		return readTextPrompts(filePath)
	}
}

// readYAMLPrompts reads prompts from a YAML file
func readYAMLPrompts(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("err_file_open"), err)
	}

	var promptFile PromptFile
	if err := yaml.Unmarshal(data, &promptFile); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file: %v", err)
	}

	var prompts []string
	for _, entry := range promptFile.Prompts {
		prompt := strings.TrimSpace(entry.Prompt)
		if prompt != "" {
			prompts = append(prompts, prompt)
		}
	}

	if len(prompts) == 0 {
		return nil, fmt.Errorf(i18n.T("err_file_empty"))
	}

	return prompts, nil
}

// readJSONLPrompts reads prompts from a JSONL file
func readJSONLPrompts(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("err_file_open"), err)
	}
	defer file.Close()

	var prompts []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var jsonPrompt JSONLPrompt
		if err := json.Unmarshal([]byte(line), &jsonPrompt); err != nil {
			return nil, fmt.Errorf("failed to parse JSONL line: %v", err)
		}

		prompt := strings.TrimSpace(jsonPrompt.Prompt)
		if prompt != "" {
			prompts = append(prompts, prompt)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf(i18n.T("err_file_read"), err)
	}

	if len(prompts) == 0 {
		return nil, fmt.Errorf(i18n.T("err_file_empty"))
	}

	return prompts, nil
}

// readTextPrompts reads prompts from a plain text file (original format)
func readTextPrompts(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("err_file_open"), err)
	}
	defer file.Close()

	var prompts []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			prompts = append(prompts, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf(i18n.T("err_file_read"), err)
	}

	if len(prompts) == 0 {
		return nil, fmt.Errorf(i18n.T("err_file_empty"))
	}

	return prompts, nil
}
