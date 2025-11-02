// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package oracle

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

// OracleProvider implements the Provider interface for Oracle Cloud Generative AI
type OracleProvider struct {
	endpoint   string // OCI endpoint
	authToken  string // OCI auth token or API key
	compartmentID string
	httpClient *http.Client
}

// Config for Oracle Cloud AI provider
type OracleConfig struct {
	Endpoint      string `yaml:"endpoint"`       // OCI endpoint URL
	AuthToken     string `yaml:"auth_token"`     // Auth token
	CompartmentID string `yaml:"compartment_id"` // OCI compartment ID
}

// Oracle Generative AI request/response types
type OracleRequest struct {
	CompartmentID string                 `json:"compartmentId"`
	ServingMode   OracleServingMode      `json:"servingMode"`
	ChatRequest   *OracleChatRequest     `json:"chatRequest,omitempty"`
}

type OracleServingMode struct {
	ServingType string `json:"servingType"` // ON_DEMAND or DEDICATED
	ModelID     string `json:"modelId,omitempty"`
}

type OracleChatRequest struct {
	Messages         []OracleMessage        `json:"messages"`
	MaxTokens        *int                   `json:"maxTokens,omitempty"`
	Temperature      *float64               `json:"temperature,omitempty"`
	TopP             *float64               `json:"topP,omitempty"`
	TopK             *int                   `json:"topK,omitempty"`
	FrequencyPenalty *float64               `json:"frequencyPenalty,omitempty"`
	PresencePenalty  *float64               `json:"presencePenalty,omitempty"`
	Stop             []string               `json:"stop,omitempty"`
}

type OracleMessage struct {
	Role    string                 `json:"role"` // USER, ASSISTANT, SYSTEM
	Content []OracleContent        `json:"content"`
}

type OracleContent struct {
	Type string `json:"type"` // TEXT
	Text string `json:"text"`
}

type OracleResponse struct {
	ChatResponse OracleChatResponse `json:"chatResponse"`
}

type OracleChatResponse struct {
	Text         string              `json:"text"`
	Choices      []OracleChoice      `json:"choices"`
	TimeCreated  string              `json:"timeCreated"`
	ModelID      string              `json:"modelId"`
	ModelVersion string              `json:"modelVersion"`
}

type OracleChoice struct {
	Index        int                  `json:"index"`
	Message      OracleMessage        `json:"message"`
	FinishReason string               `json:"finishReason"`
}

// NewOracleProvider creates a new Oracle Cloud AI provider
func NewOracleProvider(config OracleConfig) (*OracleProvider, error) {
	if config.Endpoint == "" {
		return nil, fmt.Errorf("Oracle endpoint is required")
	}
	if config.AuthToken == "" {
		return nil, fmt.Errorf("Oracle auth token is required")
	}
	if config.CompartmentID == "" {
		return nil, fmt.Errorf("Oracle compartment ID is required")
	}

	return &OracleProvider{
		endpoint:      config.Endpoint,
		authToken:     config.AuthToken,
		compartmentID: config.CompartmentID,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Name returns the provider name
func (p *OracleProvider) Name() string {
	return "oracle"
}

// HealthCheck checks if the provider is accessible
func (p *OracleProvider) HealthCheck(ctx context.Context) error {
	// Could check API availability, but skip for now
	return nil
}

// Invoke sends a request to Oracle Generative AI
func (p *OracleProvider) Invoke(ctx context.Context, request *providers.ProviderRequest) (*providers.ProviderResponse, error) {
	// Parse OpenAI request
	var openaiReq translator.ChatCompletionRequest
	if err := json.Unmarshal(request.Body, &openaiReq); err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("failed to parse request: %v", err),
			Provider:   "oracle",
		}
	}

	// Translate to Oracle format
	oracleReq := translateOpenAIToOracle(&openaiReq, p.compartmentID)

	// Marshal request
	body, err := json.Marshal(oracleReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to marshal request: %v", err),
			Provider:   "oracle",
		}
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/20231130/actions/chat", p.endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to create request: %v", err),
			Provider:   "oracle",
		}
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.authToken)

	// Send request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    fmt.Sprintf("request failed: %v", err),
			Provider:   "oracle",
		}
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to read response: %v", err),
			Provider:   "oracle",
		}
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, &providers.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
			Provider:   "oracle",
		}
	}

	// Parse Oracle response
	var oracleResp OracleResponse
	if err := json.Unmarshal(respBody, &oracleResp); err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to parse response: %v", err),
			Provider:   "oracle",
		}
	}

	// Translate back to OpenAI format
	openaiResp := translateOracleToOpenAI(&oracleResp, openaiReq.Model)

	// Marshal OpenAI response
	openaiBody, err := json.Marshal(openaiResp)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to marshal response: %v", err),
			Provider:   "oracle",
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

// InvokeStreaming sends a streaming request to Oracle
func (p *OracleProvider) InvokeStreaming(ctx context.Context, request *providers.ProviderRequest) (io.ReadCloser, error) {
	return nil, &providers.ProviderError{
		StatusCode: http.StatusNotImplemented,
		Message:    "streaming not yet implemented for Oracle provider",
		Provider:   "oracle",
	}
}

// ListModels lists available Oracle Generative AI models
func (p *OracleProvider) ListModels(ctx context.Context) ([]providers.Model, error) {
	// Hardcoded list of Oracle Cloud AI models
	models := []providers.Model{
		{ID: "cohere.command-r-plus", Name: "Cohere Command R Plus", Provider: "oracle"},
		{ID: "cohere.command-r-16k", Name: "Cohere Command R 16K", Provider: "oracle"},
		{ID: "meta.llama-3-70b-instruct", Name: "Meta Llama 3 70B Instruct", Provider: "oracle"},
		{ID: "meta.llama-2-70b-chat", Name: "Meta Llama 2 70B Chat", Provider: "oracle"},
	}

	return models, nil
}

// GetModelInfo gets information about a specific Oracle model
func (p *OracleProvider) GetModelInfo(ctx context.Context, modelID string) (*providers.Model, error) {
	models, _ := p.ListModels(ctx)
	for _, m := range models {
		if m.ID == modelID {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("model not found: %s", modelID)
}

// translateOpenAIToOracle converts OpenAI format to Oracle Cloud AI format
func translateOpenAIToOracle(req *translator.ChatCompletionRequest, compartmentID string) *OracleRequest {
	oracleReq := &OracleRequest{
		CompartmentID: compartmentID,
		ServingMode: OracleServingMode{
			ServingType: "ON_DEMAND",
			ModelID:     req.Model,
		},
		ChatRequest: &OracleChatRequest{},
	}

	// Set parameters
	if req.MaxTokens > 0 {
		oracleReq.ChatRequest.MaxTokens = &req.MaxTokens
	}
	if req.Temperature > 0 {
		oracleReq.ChatRequest.Temperature = &req.Temperature
	}
	if req.TopP > 0 {
		oracleReq.ChatRequest.TopP = &req.TopP
	}
	if req.FrequencyPenalty > 0 {
		oracleReq.ChatRequest.FrequencyPenalty = &req.FrequencyPenalty
	}
	if req.PresencePenalty > 0 {
		oracleReq.ChatRequest.PresencePenalty = &req.PresencePenalty
	}
	if len(req.Stop) > 0 {
		oracleReq.ChatRequest.Stop = req.Stop
	}

	// Convert messages
	for _, msg := range req.Messages {
		role := msg.Role
		// Map roles to Oracle format (uppercase)
		if role == "user" {
			role = "USER"
		} else if role == "assistant" {
			role = "ASSISTANT"
		} else if role == "system" {
			role = "SYSTEM"
		}

		text := extractTextContent(msg.Content)
		oracleReq.ChatRequest.Messages = append(oracleReq.ChatRequest.Messages, OracleMessage{
			Role: role,
			Content: []OracleContent{
				{Type: "TEXT", Text: text},
			},
		})
	}

	return oracleReq
}

// translateOracleToOpenAI converts Oracle response to OpenAI format
func translateOracleToOpenAI(resp *OracleResponse, model string) *translator.ChatCompletionResponse {
	var content string
	finishReason := "stop"

	if len(resp.ChatResponse.Choices) > 0 {
		choice := resp.ChatResponse.Choices[0]

		// Extract text from message
		for _, contentBlock := range choice.Message.Content {
			if contentBlock.Type == "TEXT" {
				content += contentBlock.Text
			}
		}

		// Map finish reason
		switch choice.FinishReason {
		case "FINISH", "COMPLETE":
			finishReason = "stop"
		case "LENGTH":
			finishReason = "length"
		case "CONTENT_FILTER":
			finishReason = "content_filter"
		}
	}

	return &translator.ChatCompletionResponse{
		ID:      fmt.Sprintf("oracle-%d", time.Now().Unix()),
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
			PromptTokens:     0, // Oracle doesn't always return token counts
			CompletionTokens: 0,
			TotalTokens:      0,
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
