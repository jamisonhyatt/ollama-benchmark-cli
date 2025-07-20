package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"ollama-benchmark/internal/i18n"
)

type ModelInfo struct {
	Name string `json:"name"`
}

type ModelListResponse struct {
	Models []ModelInfo `json:"models"`
}

func GetModelList(apiURL string) ([]string, error) {
	url := fmt.Sprintf("%s/api/tags", apiURL)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("err_api_request"), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf(i18n.T("err_api_status"), resp.Status)
	}

	var parsed ModelListResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf(i18n.T("err_api_parse"), err)
	}

	var modelNames []string
	for _, model := range parsed.Models {
		modelNames = append(modelNames, model.Name)
	}

	return modelNames, nil
}

// UnloadModelRequest represents the request to unload a model
type UnloadModelRequest struct {
	Model     string `json:"model"`
	KeepAlive string `json:"keep_alive"`
}

// UnloadModel tells Ollama to unload a specific model from memory
func UnloadModel(apiURL, modelName string) error {
	url := fmt.Sprintf("%s/api/generate", apiURL)

	// Setting keep_alive to "0" immediately unloads the model
	requestBody := UnloadModelRequest{
		Model:     modelName,
		KeepAlive: "0",
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal unload request: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send unload request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unload request failed with status: %s", resp.Status)
	}

	return nil
}
