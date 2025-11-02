// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package ibm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tosharewith/llmproxy_auth/internal/providers"
	"github.com/tosharewith/llmproxy_auth/internal/translator"
)

// IBMProvider implements the Provider interface for IBM watsonx.ai
type IBMProvider struct {
	apiKey     string
	projectID  string
	baseURL    string
	httpClient *http.Client
}

// Config for IBM watsonx.ai provider
type IBMConfig struct {
	APIKey    string `yaml:"api_key"`
	ProjectID string `yaml:"project_id"`
	BaseURL   string `yaml:"base_url"` // Optional, defaults to https://us-south.ml.cloud.ibm.com
}

// IBM watsonx.ai request/response types
type IBMRequest struct {
	ModelID    string                 `json:"model_id"`
	Input      string                 `json:"input"`
	Parameters *IBMParameters         `json:"parameters,omitempty"`
	ProjectID  string                 `json:"project_id"`
}

type IBMParameters struct {
	MaxNewTokens   *int     `json:"max_new_tokens,omitempty"`
	MinNewTokens   *int     `json:"min_new_tokens,omitempty"`
	Temperature    *float64 `json:"temperature,omitempty"`
	TopP           *float64 `json:"top_p,omitempty"`
	TopK           *int     `json:"top_k,omitempty"`
	StopSequences  []string `json:"stop_sequences,omitempty"`
}

type IBMResponse struct {
	ModelID    string        `json:"model_id"`
	Results    []IBMResult   `json:"results"`
	CreatedAt  string        `json:"created_at"`
}

type IBMResult struct {
	GeneratedText    string `json:"generated_text"`
	GeneratedTokens  int    `json:"generated_token_count"`
	InputTokens      int    `json:"input_token_count"`
	StopReason       string `json:"stop_reason"`
}

// NewIBMProvider creates a new IBM watsonx.ai provider
func NewIBMProvider(config IBMConfig) (*IBMProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("IBM API key is required")
	}
	if config.ProjectID == "" {
		return nil, fmt.Errorf("IBM project ID is required")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://us-south.ml.cloud.ibm.com"
	}

	return &IBMProvider{
		apiKey:    config.APIKey,
		projectID: config.ProjectID,
		baseURL:   baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Name returns the provider name
func (p *IBMProvider) Name() string {
	return "ibm"
}

// HealthCheck checks if the provider is accessible
func (p *IBMProvider) HealthCheck(ctx context.Context) error {
	// Could check API availability, but skip for now
	return nil
}

// Invoke sends a request to IBM watsonx.ai
func (p *IBMProvider) Invoke(ctx context.Context, request *providers.ProviderRequest) (*providers.ProviderResponse, error) {
	// Parse OpenAI request
	var openaiReq translator.ChatCompletionRequest
	if err := json.Unmarshal(request.Body, &openaiReq); err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("failed to parse request: %v", err),
			Provider:   "ibm",
		}
	}

	// Translate to IBM format
	ibmReq := translateOpenAIToIBM(&openaiReq, p.projectID)

	// Marshal request
	body, err := json.Marshal(ibmReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to marshal request: %v", err),
			Provider:   "ibm",
		}
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/ml/v1/text/generation?version=2023-05-29", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to create request: %v", err),
			Provider:   "ibm",
		}
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Accept", "application/json")

	// Send request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    fmt.Sprintf("request failed: %v", err),
			Provider:   "ibm",
		}
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to read response: %v", err),
			Provider:   "ibm",
		}
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, &providers.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
			Provider:   "ibm",
		}
	}

	// Parse IBM response
	var ibmResp IBMResponse
	if err := json.Unmarshal(respBody, &ibmResp); err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to parse response: %v", err),
			Provider:   "ibm",
		}
	}

	// Translate back to OpenAI format
	openaiResp := translateIBMToOpenAI(&ibmResp, openaiReq.Model)

	// Marshal OpenAI response
	openaiBody, err := json.Marshal(openaiResp)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to marshal response: %v", err),
			Provider:   "ibm",
		}
	}

	// Build provider response
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &providers.ProviderResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       openaiBody,
	}, nil
}

// InvokeStreaming sends a streaming request to IBM watsonx.ai
func (p *IBMProvider) InvokeStreaming(ctx context.Context, request *providers.ProviderRequest) (io.ReadCloser, error) {
	return nil, &providers.ProviderError{
		StatusCode: http.StatusNotImplemented,
		Message:    "streaming not yet implemented for IBM provider",
		Provider:   "ibm",
	}
}

// ListModels lists available IBM watsonx.ai models
func (p *IBMProvider) ListModels(ctx context.Context) ([]providers.Model, error) {
	// Hardcoded list of common IBM watsonx.ai models
	models := []providers.Model{
		{ID: "ibm/granite-13b-chat-v2", Name: "Granite 13B Chat v2", Provider: "ibm"},
		{ID: "ibm/granite-13b-instruct-v2", Name: "Granite 13B Instruct v2", Provider: "ibm"},
		{ID: "meta-llama/llama-3-70b-instruct", Name: "Llama 3 70B Instruct", Provider: "ibm"},
		{ID: "meta-llama/llama-3-8b-instruct", Name: "Llama 3 8B Instruct", Provider: "ibm"},
		{ID: "mistralai/mixtral-8x7b-instruct-v01", Name: "Mixtral 8x7B Instruct", Provider: "ibm"},
		{ID: "google/flan-ul2", Name: "Flan-UL2", Provider: "ibm"},
	}

	return models, nil
}

// GetModelInfo gets information about a specific IBM model
func (p *IBMProvider) GetModelInfo(ctx context.Context, modelID string) (*providers.Model, error) {
	models, _ := p.ListModels(ctx)
	for _, m := range models {
		if m.ID == modelID {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("model not found: %s", modelID)
}

// translateOpenAIToIBM converts OpenAI format to IBM watsonx.ai format
func translateOpenAIToIBM(req *translator.ChatCompletionRequest, projectID string) *IBMRequest {
	// Convert messages to a single input string
	// IBM's simple generation API doesn't support multi-turn chat directly
	var input string
	for _, msg := range req.Messages {
		text := extractTextContent(msg.Content)
		if msg.Role == "system" {
			input += fmt.Sprintf("System: %s\n\n", text)
		} else if msg.Role == "user" {
			input += fmt.Sprintf("User: %s\n\n", text)
		} else if msg.Role == "assistant" {
			input += fmt.Sprintf("Assistant: %s\n\n", text)
		}
	}
	input += "Assistant:"

	ibmReq := &IBMRequest{
		ModelID:   req.Model,
		Input:     input,
		ProjectID: projectID,
		Parameters: &IBMParameters{},
	}

	if req.MaxTokens > 0 {
		ibmReq.Parameters.MaxNewTokens = &req.MaxTokens
	}
	if req.Temperature > 0 {
		ibmReq.Parameters.Temperature = &req.Temperature
	}
	if req.TopP > 0 {
		ibmReq.Parameters.TopP = &req.TopP
	}
	if len(req.Stop) > 0 {
		ibmReq.Parameters.StopSequences = req.Stop
	}

	return ibmReq
}

// translateIBMToOpenAI converts IBM response to OpenAI format
func translateIBMToOpenAI(resp *IBMResponse, model string) *translator.ChatCompletionResponse {
	var content string
	var promptTokens, completionTokens int
	finishReason := "stop"

	if len(resp.Results) > 0 {
		result := resp.Results[0]
		content = result.GeneratedText
		promptTokens = result.InputTokens
		completionTokens = result.GeneratedTokens

		// Map stop reason
		switch result.StopReason {
		case "eos_token", "stop_sequence":
			finishReason = "stop"
		case "max_tokens":
			finishReason = "length"
		}
	}

	return &translator.ChatCompletionResponse{
		ID:      fmt.Sprintf("ibm-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []translator.ChatCompletionChoice{
			{
				Index: 0,
				Message: translator.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
		Usage: &translator.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}
}

// extractTextContent extracts text from content interface
func extractTextContent(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var text string
		for _, part := range c {
			if partMap, ok := part.(map[string]interface{}); ok {
				if partType, ok := partMap["type"].(string); ok && partType == "text" {
					if textVal, ok := partMap["text"].(string); ok {
						text += textVal
					}
				}
			}
		}
		return text
	default:
		return fmt.Sprintf("%v", content)
	}
}
