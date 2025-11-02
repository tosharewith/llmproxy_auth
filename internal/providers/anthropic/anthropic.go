// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package anthropic

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

// AnthropicProvider implements the Provider interface for Anthropic Direct API
type AnthropicProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Config for Anthropic provider
type AnthropicConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"` // Optional, defaults to https://api.anthropic.com/v1
}

// Anthropic Messages API types
type AnthropicRequest struct {
	Model       string              `json:"model"`
	Messages    []AnthropicMessage  `json:"messages"`
	MaxTokens   int                 `json:"max_tokens"`
	Temperature *float64            `json:"temperature,omitempty"`
	System      string              `json:"system,omitempty"`
	Tools       []AnthropicTool     `json:"tools,omitempty"`
	ToolChoice  interface{}         `json:"tool_choice,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
}

type AnthropicMessage struct {
	Role    string                 `json:"role"` // user or assistant
	Content interface{}            `json:"content"` // string or []ContentBlock
}

type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type AnthropicResponse struct {
	ID           string                   `json:"id"`
	Type         string                   `json:"type"` // "message"
	Role         string                   `json:"role"` // "assistant"
	Content      []AnthropicContentBlock  `json:"content"`
	Model        string                   `json:"model"`
	StopReason   string                   `json:"stop_reason"`
	Usage        AnthropicUsage           `json:"usage"`
}

type AnthropicContentBlock struct {
	Type  string                 `json:"type"` // "text" or "tool_use"
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`    // for tool_use
	Name  string                 `json:"name,omitempty"`  // for tool_use
	Input map[string]interface{} `json:"input,omitempty"` // for tool_use
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(config AnthropicConfig) (*AnthropicProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Anthropic API key is required")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	return &AnthropicProvider{
		apiKey:  config.APIKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Name returns the provider name
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// HealthCheck checks if the provider is accessible
func (p *AnthropicProvider) HealthCheck(ctx context.Context) error {
	// Anthropic doesn't have a dedicated health endpoint, so we'll skip for now
	// Could do a lightweight messages call as health check
	return nil
}

// Invoke sends a request to Anthropic
func (p *AnthropicProvider) Invoke(ctx context.Context, request *providers.ProviderRequest) (*providers.ProviderResponse, error) {
	// Parse OpenAI request
	var openaiReq translator.ChatCompletionRequest
	if err := json.Unmarshal(request.Body, &openaiReq); err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("failed to parse request: %v", err),
			Provider:   "anthropic",
		}
	}

	// Translate to Anthropic format
	anthropicReq := translateOpenAIToAnthropic(&openaiReq)

	// Marshal request
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to marshal request: %v", err),
			Provider:   "anthropic",
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to create request: %v", err),
			Provider:   "anthropic",
		}
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Send request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    fmt.Sprintf("request failed: %v", err),
			Provider:   "anthropic",
		}
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to read response: %v", err),
			Provider:   "anthropic",
		}
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, &providers.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
			Provider:   "anthropic",
		}
	}

	// Parse Anthropic response
	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to parse response: %v", err),
			Provider:   "anthropic",
		}
	}

	// Translate back to OpenAI format
	openaiResp := translateAnthropicToOpenAI(&anthropicResp, openaiReq.Model)

	// Marshal OpenAI response
	openaiBody, err := json.Marshal(openaiResp)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to marshal response: %v", err),
			Provider:   "anthropic",
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

// InvokeStreaming sends a streaming request to Anthropic
func (p *AnthropicProvider) InvokeStreaming(ctx context.Context, request *providers.ProviderRequest) (io.ReadCloser, error) {
	// Parse and translate request
	var openaiReq translator.ChatCompletionRequest
	if err := json.Unmarshal(request.Body, &openaiReq); err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("failed to parse request: %v", err),
			Provider:   "anthropic",
		}
	}

	anthropicReq := translateOpenAIToAnthropic(&openaiReq)
	anthropicReq.Stream = true

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to marshal request: %v", err),
			Provider:   "anthropic",
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to create request: %v", err),
			Provider:   "anthropic",
		}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    fmt.Sprintf("request failed: %v", err),
			Provider:   "anthropic",
		}
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, &providers.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Provider:   "anthropic",
		}
	}

	return resp.Body, nil
}

// ListModels lists available Anthropic models
func (p *AnthropicProvider) ListModels(ctx context.Context) ([]providers.Model, error) {
	// Anthropic doesn't have a models API endpoint yet, return hardcoded list
	models := []providers.Model{
		{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Provider: "anthropic"},
		{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Provider: "anthropic"},
		{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: "anthropic"},
		{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet", Provider: "anthropic"},
		{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Provider: "anthropic"},
	}

	return models, nil
}

// GetModelInfo gets information about a specific Anthropic model
func (p *AnthropicProvider) GetModelInfo(ctx context.Context, modelID string) (*providers.Model, error) {
	models, _ := p.ListModels(ctx)
	for _, m := range models {
		if m.ID == modelID {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("model not found: %s", modelID)
}

// translateOpenAIToAnthropic converts OpenAI format to Anthropic format
func translateOpenAIToAnthropic(req *translator.ChatCompletionRequest) *AnthropicRequest {
	anthropicReq := &AnthropicRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
	}

	if req.Temperature > 0 {
		anthropicReq.Temperature = &req.Temperature
	}

	// Convert messages
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Extract system message
			anthropicReq.System = extractTextContent(msg.Content)
		} else {
			// User and assistant messages
			anthropicReq.Messages = append(anthropicReq.Messages, AnthropicMessage{
				Role:    msg.Role,
				Content: extractTextContent(msg.Content),
			})
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		for _, tool := range req.Tools {
			if tool.Type == "function" {
				anthropicReq.Tools = append(anthropicReq.Tools, AnthropicTool{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					InputSchema: tool.Function.Parameters,
				})
			}
		}
	}

	// Convert tool_choice
	if req.ToolChoice != nil {
		switch tc := req.ToolChoice.(type) {
		case string:
			if tc == "auto" {
				anthropicReq.ToolChoice = map[string]string{"type": "auto"}
			} else if tc == "required" || tc == "any" {
				anthropicReq.ToolChoice = map[string]string{"type": "any"}
			}
		case map[string]interface{}:
			if tcType, ok := tc["type"].(string); ok && tcType == "function" {
				if function, ok := tc["function"].(map[string]interface{}); ok {
					if name, ok := function["name"].(string); ok {
						anthropicReq.ToolChoice = map[string]interface{}{
							"type": "tool",
							"name": name,
						}
					}
				}
			}
		}
	}

	return anthropicReq
}

// translateAnthropicToOpenAI converts Anthropic response to OpenAI format
func translateAnthropicToOpenAI(resp *AnthropicResponse, model string) *translator.ChatCompletionResponse {
	var content string
	var toolCalls []translator.ToolCall

	// Extract content and tool calls
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		} else if block.Type == "tool_use" {
			argsJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, translator.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: translator.FunctionCall{
					Name:      block.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}

	// Map stop reason
	finishReason := "stop"
	if resp.StopReason == "max_tokens" {
		finishReason = "length"
	} else if resp.StopReason == "tool_use" {
		finishReason = "tool_calls"
	}

	message := translator.ChatMessage{
		Role:    "assistant",
		Content: content,
	}

	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	return &translator.ChatCompletionResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []translator.ChatCompletionChoice{
			{
				Index:        0,
				Message:      message,
				FinishReason: finishReason,
			},
		},
		Usage: &translator.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

// extractTextContent extracts text content from OpenAI message content
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
