// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package bedrock

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/tosharewith/llmproxy_auth/internal/auth"
	"github.com/tosharewith/llmproxy_auth/internal/providers"
)

// BedrockProvider implements the Provider interface for AWS Bedrock
type BedrockProvider struct {
	region    string
	baseURL   string
	signer    *auth.AWSSigner
	httpClient *http.Client
}

// NewBedrockProvider creates a new Bedrock provider
func NewBedrockProvider(region string) (*BedrockProvider, error) {
	// Create AWS signer
	signer, err := auth.NewAWSSigner(region, "bedrock")
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS signer: %w", err)
	}

	// Create HTTP client with reasonable timeout
	httpClient := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	baseURL := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", region)

	return &BedrockProvider{
		region:     region,
		baseURL:    baseURL,
		signer:     signer,
		httpClient: httpClient,
	}, nil
}

// Name returns the provider identifier
func (p *BedrockProvider) Name() string {
	return "bedrock"
}

// HealthCheck verifies the provider is accessible
func (p *BedrockProvider) HealthCheck(ctx context.Context) error {
	// Simple health check - try to list foundation models
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/foundation-models", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	// Sign the request
	if err := p.signer.SignRequest(req, nil); err != nil {
		return fmt.Errorf("failed to sign health check request: %w", err)
	}

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// Invoke sends a request to Bedrock
func (p *BedrockProvider) Invoke(ctx context.Context, request *providers.ProviderRequest) (*providers.ProviderResponse, error) {
	startTime := time.Now()

	// Build full URL
	url := p.baseURL + request.Path

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, request.Method, url, bytes.NewReader(request.Body))
	if err != nil {
		return nil, &providers.ProviderError{
			Provider:   p.Name(),
			Code:       providers.ErrCodeInternalError,
			Message:    "Failed to create request",
			Err:        err,
		}
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}

	// Add query parameters
	if len(request.QueryParams) > 0 {
		q := req.URL.Query()
		for key, value := range request.QueryParams {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	// Sign the request with AWS Signature V4
	if err := p.signer.SignRequest(req, request.Body); err != nil {
		return nil, &providers.ProviderError{
			Provider:   p.Name(),
			Code:       providers.ErrCodeAuthenticationFail,
			Message:    "Failed to sign request",
			Err:        err,
		}
	}

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider:   p.Name(),
			Code:       providers.ErrCodeServiceUnavailable,
			Message:    "Request failed",
			Err:        err,
		}
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider:   p.Name(),
			Code:       providers.ErrCodeInternalError,
			Message:    "Failed to read response",
			Err:        err,
		}
	}

	// Handle error responses
	if resp.StatusCode >= 400 {
		return nil, p.handleErrorResponse(resp.StatusCode, respBody)
	}

	// Build response
	latency := time.Since(startTime)
	response := &providers.ProviderResponse{
		StatusCode: resp.StatusCode,
		Headers:    make(map[string]string),
		Body:       respBody,
		Metadata: providers.ResponseMetadata{
			Latency:    latency,
			ModelUsed:  extractModelFromPath(request.Path),
		},
	}

	// Copy response headers
	for key := range resp.Header {
		response.Headers[key] = resp.Header.Get(key)
	}

	return response, nil
}

// InvokeStreaming handles streaming responses
func (p *BedrockProvider) InvokeStreaming(ctx context.Context, request *providers.ProviderRequest) (io.ReadCloser, error) {
	// Build full URL
	url := p.baseURL + request.Path

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, request.Method, url, bytes.NewReader(request.Body))
	if err != nil {
		return nil, &providers.ProviderError{
			Provider:   p.Name(),
			Code:       providers.ErrCodeInternalError,
			Message:    "Failed to create streaming request",
			Err:        err,
		}
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.amazon.eventstream")
	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}

	// Sign the request
	if err := p.signer.SignRequest(req, request.Body); err != nil {
		return nil, &providers.ProviderError{
			Provider:   p.Name(),
			Code:       providers.ErrCodeAuthenticationFail,
			Message:    "Failed to sign streaming request",
			Err:        err,
		}
	}

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider:   p.Name(),
			Code:       providers.ErrCodeServiceUnavailable,
			Message:    "Streaming request failed",
			Err:        err,
		}
	}

	// Check for error status
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, p.handleErrorResponse(resp.StatusCode, body)
	}

	// Return the response body as a ReadCloser
	return resp.Body, nil
}

// ListModels returns available Bedrock models
func (p *BedrockProvider) ListModels(ctx context.Context) ([]providers.Model, error) {
	return BedrockModels, nil
}

// GetModelInfo returns information about a specific model
func (p *BedrockProvider) GetModelInfo(ctx context.Context, modelID string) (*providers.Model, error) {
	modelInfo := GetBedrockModelInfo(modelID)
	if modelInfo == nil {
		return nil, &providers.ProviderError{
			Provider:   p.Name(),
			Code:       providers.ErrCodeModelNotFound,
			Message:    fmt.Sprintf("Model %q not found", modelID),
		}
	}
	return modelInfo, nil
}

// handleErrorResponse converts Bedrock error responses to ProviderError
func (p *BedrockProvider) handleErrorResponse(statusCode int, body []byte) error {
	var code string
	var message string

	switch statusCode {
	case http.StatusBadRequest:
		code = providers.ErrCodeInvalidRequest
		message = "Invalid request"
	case http.StatusUnauthorized, http.StatusForbidden:
		code = providers.ErrCodeAuthenticationFail
		message = "Authentication failed"
	case http.StatusTooManyRequests:
		code = providers.ErrCodeRateLimitExceeded
		message = "Rate limit exceeded"
	case http.StatusNotFound:
		code = providers.ErrCodeModelNotFound
		message = "Model not found"
	case http.StatusServiceUnavailable:
		code = providers.ErrCodeServiceUnavailable
		message = "Service unavailable"
	default:
		code = providers.ErrCodeInternalError
		message = "Internal server error"
	}

	// Log the actual error body for debugging
	if len(body) > 0 {
		log.Printf("Bedrock error (%d): %s", statusCode, string(body))
	}

	return &providers.ProviderError{
		Provider:   p.Name(),
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
	}
}

// extractModelFromPath extracts the model ID from the request path
func extractModelFromPath(path string) string {
	// Example path: /model/anthropic.claude-3-sonnet-20240229-v1:0/invoke
	// Extract the model ID between /model/ and /invoke
	const modelPrefix = "/model/"
	const invokeSuffix = "/invoke"

	startIdx := len(modelPrefix)
	endIdx := len(path)

	if idx := bytes.Index([]byte(path), []byte(invokeSuffix)); idx > 0 {
		endIdx = idx
	}

	if startIdx < len(path) && endIdx <= len(path) && startIdx < endIdx {
		return path[startIdx:endIdx]
	}

	return ""
}
