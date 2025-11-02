// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package azure

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

// AzureProvider implements the Provider interface for Azure OpenAI
type AzureProvider struct {
	endpoint   string      // Azure endpoint (e.g., https://your-resource.openai.azure.com)
	apiKey     string      // Azure API key
	apiVersion string      // API version (e.g., 2024-02-15-preview)
	httpClient *http.Client
}

// Config for Azure OpenAI provider
type AzureConfig struct {
	Endpoint   string `yaml:"endpoint"`   // Azure OpenAI endpoint
	APIKey     string `yaml:"api_key"`    // Azure API key
	APIVersion string `yaml:"api_version"` // API version
}

// NewAzureProvider creates a new Azure OpenAI provider
func NewAzureProvider(config AzureConfig) (*AzureProvider, error) {
	if config.Endpoint == "" {
		return nil, fmt.Errorf("Azure endpoint is required")
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("Azure API key is required")
	}
	if config.APIVersion == "" {
		config.APIVersion = "2024-02-15-preview" // Default to latest
	}

	return &AzureProvider{
		endpoint:   config.Endpoint,
		apiKey:     config.APIKey,
		apiVersion: config.APIVersion,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Name returns the provider name
func (p *AzureProvider) Name() string {
	return "azure"
}

// HealthCheck checks if the provider is accessible
func (p *AzureProvider) HealthCheck(ctx context.Context) error {
	// Try to list deployments as a health check
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/openai/deployments?api-version=%s", p.endpoint, p.apiVersion), nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("api-key", p.apiKey)

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

// Invoke sends a request to Azure OpenAI
func (p *AzureProvider) Invoke(ctx context.Context, request *providers.ProviderRequest) (*providers.ProviderResponse, error) {
	// Azure uses deployment names instead of model names
	// The path should be /openai/deployments/{deployment-id}/chat/completions
	deploymentID := extractDeploymentID(request.Path)
	if deploymentID == "" {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusBadRequest,
			Message:    "deployment ID is required for Azure",
			Provider:   "azure",
		}
	}

	// Build Azure-specific URL
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		p.endpoint, deploymentID, p.apiVersion)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, request.Method, url, bytes.NewReader(request.Body))
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to create request: %v", err),
			Provider:   "azure",
		}
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.apiKey)

	// Send request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    fmt.Sprintf("request failed: %v", err),
			Provider:   "azure",
		}
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to read response: %v", err),
			Provider:   "azure",
		}
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, &providers.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Provider:   "azure",
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

// InvokeStreaming sends a streaming request to Azure OpenAI
func (p *AzureProvider) InvokeStreaming(ctx context.Context, request *providers.ProviderRequest) (io.ReadCloser, error) {
	deploymentID := extractDeploymentID(request.Path)
	if deploymentID == "" {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusBadRequest,
			Message:    "deployment ID is required for Azure",
			Provider:   "azure",
		}
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		p.endpoint, deploymentID, p.apiVersion)

	httpReq, err := http.NewRequestWithContext(ctx, request.Method, url, bytes.NewReader(request.Body))
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to create request: %v", err),
			Provider:   "azure",
		}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, &providers.ProviderError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    fmt.Sprintf("request failed: %v", err),
			Provider:   "azure",
		}
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, &providers.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Provider:   "azure",
		}
	}

	return resp.Body, nil
}

// ListModels lists available Azure OpenAI deployments
func (p *AzureProvider) ListModels(ctx context.Context) ([]providers.Model, error) {
	url := fmt.Sprintf("%s/openai/deployments?api-version=%s", p.endpoint, p.apiVersion)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("api-key", p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var deployments struct {
		Data []struct {
			ID    string `json:"id"`
			Model string `json:"model"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&deployments); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]providers.Model, len(deployments.Data))
	for i, dep := range deployments.Data {
		models[i] = providers.Model{
			ID:       dep.ID,
			Name:     dep.Model,
			Provider: "azure",
		}
	}

	return models, nil
}

// GetModelInfo gets information about a specific Azure deployment
func (p *AzureProvider) GetModelInfo(ctx context.Context, modelID string) (*providers.Model, error) {
	url := fmt.Sprintf("%s/openai/deployments/%s?api-version=%s", p.endpoint, modelID, p.apiVersion)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("api-key", p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("deployment not found: %s", modelID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var deployment struct {
		ID    string `json:"id"`
		Model string `json:"model"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&deployment); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &providers.Model{
		ID:       deployment.ID,
		Name:     deployment.Model,
		Provider: "azure",
	}, nil
}

// extractDeploymentID extracts the deployment ID from the request path or metadata
func extractDeploymentID(path string) string {
	// Path could be /v1/chat/completions with deployment in metadata
	// Or could already contain deployment ID
	// For now, we'll expect it in the path or return empty
	// This will be populated by the router based on model mapping
	return ""
}
