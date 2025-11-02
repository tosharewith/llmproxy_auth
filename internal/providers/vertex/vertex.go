// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package vertex

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

// VertexProvider implements the Provider interface for Google Vertex AI
type VertexProvider struct {
	projectID   string
	location    string
	accessToken string // OAuth2 access token
	baseURL     string
	httpClient  *http.Client
}

// Config for Vertex AI provider
type VertexConfig struct {
	ProjectID   string `yaml:"project_id"`
	Location    string `yaml:"location"` // e.g., us-central1
	AccessToken string `yaml:"access_token"` // OAuth2 token (or use Application Default Credentials)
}

// Vertex AI Gemini API request/response types
type VertexGeminiRequest struct {
	Contents         []VertexContent       `json:"contents"`
	SystemInstruction *VertexContent       `json:"systemInstruction,omitempty"`
	Tools            []VertexTool          `json:"tools,omitempty"`
	GenerationConfig *GenerationConfig     `json:"generationConfig,omitempty"`
}

type VertexContent struct {
	Role  string        `json:"role"` // user, model
	Parts []VertexPart  `json:"parts"`
}

type VertexPart struct {
	Text         string                 `json:"text,omitempty"`
	FunctionCall *VertexFunctionCall    `json:"functionCall,omitempty"`
	FunctionResponse *VertexFunctionResponse `json:"functionResponse,omitempty"`
}

type VertexFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type VertexFunctionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

type VertexTool struct {
	FunctionDeclarations []VertexFunctionDeclaration `json:"functionDeclarations"`
}

type VertexFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type GenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type VertexResponse struct {
	Candidates     []VertexCandidate    `json:"candidates"`
	UsageMetadata  *VertexUsageMetadata `json:"usageMetadata,omitempty"`
}

type VertexCandidate struct {
	Content      VertexContent `json:"content"`
	FinishReason string        `json:"finishReason"`
	Index        int           `json:"index"`
}

type VertexUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// NewVertexProvider creates a new Vertex AI provider
func NewVertexProvider(config VertexConfig) (*VertexProvider, error) {
	if config.ProjectID == "" {
		return nil, fmt.Errorf("Vertex AI project ID is required")
	}
	if config.Location == "" {
		config.Location = "us-central1" // Default location
	}

	baseURL := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s",
		config.Location, config.ProjectID, config.Location)

	return &VertexProvider{
		projectID:   config.ProjectID,
		location:    config.Location,
		accessToken: config.AccessToken,
		baseURL:     baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Name returns the provider name
func (p *VertexProvider) Name() string {
	return "vertex"
}

// HealthCheck checks if the provider is accessible
func (p *VertexProvider) HealthCheck(ctx context.Context) error {
	// Could list models or endpoints, but skip for now
	return nil
}

// Invoke sends a request to Vertex AI
func (p *VertexProvider) Invoke(ctx context.Context, request *providers.ProviderRequest) (*providers.ProviderResponse, error) {
	// Parse OpenAI request
	var openaiReq translator.ChatCompletionRequest
	if err := json.Unmarshal(request.Body, &openaiReq); err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("failed to parse request: %v", err),
			Provider:   "vertex",
		}
	}

	// Translate to Vertex format
	vertexReq := translateOpenAIToVertex(&openaiReq)

	// Marshal request
	body, err := json.Marshal(vertexReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to marshal request: %v", err),
			Provider:   "vertex",
		}
	}

	// Build URL based on model
	// For Gemini: /publishers/google/models/{model}:generateContent
	modelID := openaiReq.Model
	url := fmt.Sprintf("%s/publishers/google/models/%s:generateContent", p.baseURL, modelID)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to create request: %v", err),
			Provider:   "vertex",
		}
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if p.accessToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.accessToken)
	}

	// Send request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    fmt.Sprintf("request failed: %v", err),
			Provider:   "vertex",
		}
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to read response: %v", err),
			Provider:   "vertex",
		}
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, &providers.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
			Provider:   "vertex",
		}
	}

	// Parse Vertex response
	var vertexResp VertexResponse
	if err := json.Unmarshal(respBody, &vertexResp); err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to parse response: %v", err),
			Provider:   "vertex",
		}
	}

	// Translate back to OpenAI format
	openaiResp := translateVertexToOpenAI(&vertexResp, openaiReq.Model)

	// Marshal OpenAI response
	openaiBody, err := json.Marshal(openaiResp)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to marshal response: %v", err),
			Provider:   "vertex",
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

// InvokeStreaming sends a streaming request to Vertex AI
func (p *VertexProvider) InvokeStreaming(ctx context.Context, request *providers.ProviderRequest) (io.ReadCloser, error) {
	var openaiReq translator.ChatCompletionRequest
	if err := json.Unmarshal(request.Body, &openaiReq); err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("failed to parse request: %v", err),
			Provider:   "vertex",
		}
	}

	vertexReq := translateOpenAIToVertex(&openaiReq)
	body, err := json.Marshal(vertexReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to marshal request: %v", err),
			Provider:   "vertex",
		}
	}

	modelID := openaiReq.Model
	url := fmt.Sprintf("%s/publishers/google/models/%s:streamGenerateContent", p.baseURL, modelID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to create request: %v", err),
			Provider:   "vertex",
		}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.accessToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.accessToken)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    fmt.Sprintf("request failed: %v", err),
			Provider:   "vertex",
		}
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, &providers.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Provider:   "vertex",
		}
	}

	return resp.Body, nil
}

// ListModels lists available Vertex AI models
func (p *VertexProvider) ListModels(ctx context.Context) ([]providers.Model, error) {
	// Hardcoded list of common Vertex AI models
	models := []providers.Model{
		{ID: "gemini-1.5-pro", Name: "Gemini 1.5 Pro", Provider: "vertex"},
		{ID: "gemini-1.5-flash", Name: "Gemini 1.5 Flash", Provider: "vertex"},
		{ID: "gemini-1.0-pro", Name: "Gemini 1.0 Pro", Provider: "vertex"},
		{ID: "gemini-pro", Name: "Gemini Pro", Provider: "vertex"},
		{ID: "gemini-pro-vision", Name: "Gemini Pro Vision", Provider: "vertex"},
		{ID: "text-bison", Name: "PaLM 2 Text Bison", Provider: "vertex"},
		{ID: "chat-bison", Name: "PaLM 2 Chat Bison", Provider: "vertex"},
	}

	return models, nil
}

// GetModelInfo gets information about a specific Vertex AI model
func (p *VertexProvider) GetModelInfo(ctx context.Context, modelID string) (*providers.Model, error) {
	models, _ := p.ListModels(ctx)
	for _, m := range models {
		if m.ID == modelID {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("model not found: %s", modelID)
}

// translateOpenAIToVertex converts OpenAI format to Vertex AI format
func translateOpenAIToVertex(req *translator.ChatCompletionRequest) *VertexGeminiRequest {
	vertexReq := &VertexGeminiRequest{
		GenerationConfig: &GenerationConfig{},
	}

	// Set generation config
	if req.Temperature > 0 {
		vertexReq.GenerationConfig.Temperature = &req.Temperature
	}
	if req.TopP > 0 {
		vertexReq.GenerationConfig.TopP = &req.TopP
	}
	if req.MaxTokens > 0 {
		vertexReq.GenerationConfig.MaxOutputTokens = &req.MaxTokens
	}
	if len(req.Stop) > 0 {
		vertexReq.GenerationConfig.StopSequences = req.Stop
	}

	// Convert messages
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// System message goes to systemInstruction
			vertexReq.SystemInstruction = &VertexContent{
				Role: "user", // Vertex uses "user" for system instructions
				Parts: []VertexPart{
					{Text: extractTextContent(msg.Content)},
				},
			}
		} else {
			// Map roles: assistant -> model
			role := msg.Role
			if role == "assistant" {
				role = "model"
			}

			vertexReq.Contents = append(vertexReq.Contents, VertexContent{
				Role: role,
				Parts: []VertexPart{
					{Text: extractTextContent(msg.Content)},
				},
			})
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		var functionDeclarations []VertexFunctionDeclaration
		for _, tool := range req.Tools {
			if tool.Type == "function" {
				functionDeclarations = append(functionDeclarations, VertexFunctionDeclaration{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				})
			}
		}
		if len(functionDeclarations) > 0 {
			vertexReq.Tools = []VertexTool{
				{FunctionDeclarations: functionDeclarations},
			}
		}
	}

	return vertexReq
}

// translateVertexToOpenAI converts Vertex AI response to OpenAI format
func translateVertexToOpenAI(resp *VertexResponse, model string) *translator.ChatCompletionResponse {
	var content string
	var toolCalls []translator.ToolCall
	finishReason := "stop"

	if len(resp.Candidates) > 0 {
		candidate := resp.Candidates[0]

		// Extract content and function calls
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				content += part.Text
			}
			if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, translator.ToolCall{
					ID:   fmt.Sprintf("call_%d", len(toolCalls)),
					Type: "function",
					Function: translator.FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}

		// Map finish reason
		switch candidate.FinishReason {
		case "STOP":
			finishReason = "stop"
		case "MAX_TOKENS":
			finishReason = "length"
		case "SAFETY":
			finishReason = "content_filter"
		}

		if len(toolCalls) > 0 {
			finishReason = "tool_calls"
		}
	}

	message := translator.ChatMessage{
		Role:    "assistant",
		Content: content,
	}

	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	// Build usage
	usage := &translator.Usage{}
	if resp.UsageMetadata != nil {
		usage.PromptTokens = resp.UsageMetadata.PromptTokenCount
		usage.CompletionTokens = resp.UsageMetadata.CandidatesTokenCount
		usage.TotalTokens = resp.UsageMetadata.TotalTokenCount
	}

	return &translator.ChatCompletionResponse{
		ID:      fmt.Sprintf("vertex-%d", time.Now().Unix()),
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
		Usage: usage,
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
