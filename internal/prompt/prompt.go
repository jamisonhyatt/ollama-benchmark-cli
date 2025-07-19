package prompt

import (
	"bufio"
	_ "embed"
	"fmt"
	"ollama-benchmark/internal/i18n"
	"os"
	"strings"
)

//go:embed default_prompts.txt
var defaultPromptsData string

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
func ReadPromptsFromFile(filePath string) ([]string, error) {
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
