// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tosharewith/llmproxy_auth/internal/providers"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Config for OpenAI provider
type OpenAIConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"` // Optional, defaults to https://api.openai.com/v1
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(config OpenAIConfig) (*OpenAIProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OpenAIProvider{
		apiKey:  config.APIKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// HealthCheck checks if the provider is accessible
func (p *OpenAIProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Invoke sends a request to OpenAI
func (p *OpenAIProvider) Invoke(ctx context.Context, request *providers.ProviderRequest) (*providers.ProviderResponse, error) {
	// Build full URL
	url := p.baseURL + request.Path

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, request.Method, url, bytes.NewReader(request.Body))
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to create request: %v", err),
			Provider:   "openai",
		}
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Add custom headers from request
	for k, v := range request.Headers {
		httpReq.Header.Set(k, v)
	}

	// Send request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    fmt.Sprintf("request failed: %v", err),
			Provider:   "openai",
		}
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to read response: %v", err),
			Provider:   "openai",
		}
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, &providers.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Provider:   "openai",
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
		Body:       body,
	}, nil
}

// InvokeStreaming sends a streaming request to OpenAI
func (p *OpenAIProvider) InvokeStreaming(ctx context.Context, request *providers.ProviderRequest) (io.ReadCloser, error) {
	url := p.baseURL + request.Path

	httpReq, err := http.NewRequestWithContext(ctx, request.Method, url, bytes.NewReader(request.Body))
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to create request: %v", err),
			Provider:   "openai",
		}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	for k, v := range request.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    fmt.Sprintf("request failed: %v", err),
			Provider:   "openai",
		}
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, &providers.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Provider:   "openai",
		}
	}

	return resp.Body, nil
}

// ListModels lists available OpenAI models
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]providers.Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var modelsResp struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]providers.Model, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		models[i] = providers.Model{
			ID:       m.ID,
			Name:     m.ID,
			Provider: "openai",
		}
	}

	return models, nil
}

// GetModelInfo gets information about a specific OpenAI model
func (p *OpenAIProvider) GetModelInfo(ctx context.Context, modelID string) (*providers.Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models/"+modelID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var model struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&model); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &providers.Model{
		ID:       model.ID,
		Name:     model.ID,
		Provider: "openai",
	}, nil
}
