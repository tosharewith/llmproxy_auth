// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/tosharewith/llmproxy_auth/internal/providers"
	"github.com/tosharewith/llmproxy_auth/internal/router"
	"github.com/tosharewith/llmproxy_auth/internal/translator"
	"github.com/tosharewith/llmproxy_auth/pkg/metrics"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// OpenAIHandler handles OpenAI-compatible API requests
type OpenAIHandler struct {
	router *router.Router
}

// NewOpenAIHandler creates a new OpenAI handler
func NewOpenAIHandler(r *router.Router) *OpenAIHandler {
	return &OpenAIHandler{
		router: r,
	}
}

// ChatCompletions handles POST /v1/chat/completions
func (h *OpenAIHandler) ChatCompletions(c *gin.Context) {
	startTime := time.Now()

	// Parse request
	var req translator.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, translator.ErrorResponse{
			Error: translator.ErrorDetail{
				Message: "Invalid request body",
				Type:    "invalid_request_error",
				Code:    "invalid_json",
			},
		})
		return
	}

	// Validate model is specified
	if req.Model == "" {
		c.JSON(http.StatusBadRequest, translator.ErrorResponse{
			Error: translator.ErrorDetail{
				Message: "Model is required",
				Type:    "invalid_request_error",
				Code:    "missing_model",
			},
		})
		return
	}

	// Generate request ID
	requestID := fmt.Sprintf("chatcmpl-%s", uuid.New().String()[:8])

	// Set default values
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}
	if req.Temperature == 0 {
		req.Temperature = 1.0
	}

	// Route to appropriate provider
	provider, modelInfo, err := h.router.RouteRequest(c.Request.Context(), req.Model, "")
	if err != nil {
		log.Printf("Routing error for model %s: %v", req.Model, err)
		c.JSON(http.StatusBadRequest, translator.ErrorResponse{
			Error: translator.ErrorDetail{
				Message: fmt.Sprintf("Model %q not found or not available", req.Model),
				Type:    "invalid_request_error",
				Code:    "model_not_found",
			},
		})
		return
	}

	log.Printf("Routing model %s to provider %s (model: %s)", req.Model, provider.Name(), modelInfo.Model)

	// Handle streaming vs non-streaming
	if req.Stream {
		h.handleStreamingRequest(c, provider, &req, modelInfo, requestID)
	} else {
		h.handleNonStreamingRequest(c, provider, &req, modelInfo, requestID, startTime)
	}
}

// handleNonStreamingRequest handles non-streaming chat completion
func (h *OpenAIHandler) handleNonStreamingRequest(
	c *gin.Context,
	provider providers.Provider,
	req *translator.ChatCompletionRequest,
	modelInfo *router.ProviderModelInfo,
	requestID string,
	startTime time.Time,
) {
	// Translate OpenAI request to provider format
	var providerReq *providers.ProviderRequest
	var err error

	providerName := provider.Name()

	if providerName == "bedrock" {
		// Bedrock uses Converse API
		providerReq, _, err = translator.TranslateOpenAIToConverseAPI(req)
		if err != nil {
			log.Printf("Translation error: %v", err)
			c.JSON(http.StatusBadRequest, translator.ErrorResponse{
				Error: translator.ErrorDetail{
					Message: fmt.Sprintf("Failed to translate request: %v", err),
					Type:    "invalid_request_error",
					Code:    "translation_failed",
				},
			})
			return
		}
	} else if providerName == "openai" || providerName == "azure" {
		// OpenAI and Azure speak OpenAI natively - pass through
		reqBody, err := json.Marshal(req)
		if err != nil {
			log.Printf("Failed to marshal request: %v", err)
			c.JSON(http.StatusBadRequest, translator.ErrorResponse{
				Error: translator.ErrorDetail{
					Message: "Failed to marshal request",
					Type:    "invalid_request_error",
					Code:    "marshal_failed",
				},
			})
			return
		}
		providerReq = &providers.ProviderRequest{
			Method: "POST",
			Path:   "/chat/completions",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body:    reqBody,
			Context: c.Request.Context(),
		}
	} else {
		// Anthropic, Vertex, IBM, Oracle handle translation in their Invoke method
		reqBody, err := json.Marshal(req)
		if err != nil {
			log.Printf("Failed to marshal request: %v", err)
			c.JSON(http.StatusBadRequest, translator.ErrorResponse{
				Error: translator.ErrorDetail{
					Message: "Failed to marshal request",
					Type:    "invalid_request_error",
					Code:    "marshal_failed",
				},
			})
			return
		}
		providerReq = &providers.ProviderRequest{
			Method: "POST",
			Path:   "/chat/completions",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body:    reqBody,
			Context: c.Request.Context(),
		}
	}

	// Invoke provider
	providerResp, err := provider.Invoke(c.Request.Context(), providerReq)
	if err != nil {
		log.Printf("Provider invocation error: %v", err)
		h.handleProviderError(c, err)
		return
	}

	// Parse provider response and translate if needed
	var openaiResp *translator.ChatCompletionResponse

	if providerName == "bedrock" {
		// Bedrock returns Converse API format - translate to OpenAI
		var converseResp translator.ConverseResponse
		if err := json.Unmarshal(providerResp.Body, &converseResp); err != nil {
			log.Printf("Failed to parse Bedrock response: %v", err)
			c.JSON(http.StatusInternalServerError, translator.ErrorResponse{
				Error: translator.ErrorDetail{
					Message: "Failed to parse provider response",
					Type:    "internal_error",
					Code:    "response_parse_error",
				},
			})
			return
		}
		openaiResp = translator.TranslateConverseToOpenAI(&converseResp, req.Model, requestID)
	} else {
		// OpenAI, Azure, Anthropic, Vertex, IBM, Oracle return OpenAI format (or already translated)
		if err := json.Unmarshal(providerResp.Body, &openaiResp); err != nil {
			log.Printf("Failed to parse provider response: %v", err)
			c.JSON(http.StatusInternalServerError, translator.ErrorResponse{
				Error: translator.ErrorDetail{
					Message: "Failed to parse provider response",
					Type:    "internal_error",
					Code:    "response_parse_error",
				},
			})
			return
		}
	}

	// Set metadata
	openaiResp.ID = requestID
	openaiResp.Created = startTime.Unix()

	// Record metrics
	duration := time.Since(startTime)
	metrics.RequestDuration.WithLabelValues("POST", "200").Observe(duration.Seconds())
	metrics.RequestsTotal.WithLabelValues("POST", "200").Inc()

	c.JSON(http.StatusOK, openaiResp)
}

// handleStreamingRequest handles streaming chat completion
func (h *OpenAIHandler) handleStreamingRequest(
	c *gin.Context,
	provider providers.Provider,
	req *translator.ChatCompletionRequest,
	modelInfo *router.ProviderModelInfo,
	requestID string,
) {
	// TODO: Implement streaming support
	c.JSON(http.StatusNotImplemented, translator.ErrorResponse{
		Error: translator.ErrorDetail{
			Message: "Streaming not yet implemented",
			Type:    "not_implemented_error",
			Code:    "streaming_not_implemented",
		},
	})
}

// handleProviderError converts provider errors to OpenAI error format
func (h *OpenAIHandler) handleProviderError(c *gin.Context, err error) {
	if providerErr, ok := err.(*providers.ProviderError); ok {
		statusCode := providerErr.StatusCode
		if statusCode == 0 {
			statusCode = http.StatusInternalServerError
		}

		errorType := "api_error"
		switch providerErr.Code {
		case providers.ErrCodeInvalidRequest:
			errorType = "invalid_request_error"
		case providers.ErrCodeAuthenticationFail:
			errorType = "authentication_error"
		case providers.ErrCodeRateLimitExceeded:
			errorType = "rate_limit_error"
		case providers.ErrCodeModelNotFound:
			errorType = "invalid_request_error"
		}

		c.JSON(statusCode, translator.ErrorResponse{
			Error: translator.ErrorDetail{
				Message: providerErr.Message,
				Type:    errorType,
				Code:    providerErr.Code,
			},
		})
		return
	}

	// Generic error
	c.JSON(http.StatusInternalServerError, translator.ErrorResponse{
		Error: translator.ErrorDetail{
			Message: "Internal server error",
			Type:    "api_error",
			Code:    "internal_error",
		},
	})
}

// ListModels handles GET /v1/models
func (h *OpenAIHandler) ListModels(c *gin.Context) {
	models, err := h.router.ListModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, translator.ErrorResponse{
			Error: translator.ErrorDetail{
				Message: "Failed to list models",
				Type:    "api_error",
				Code:    "list_models_failed",
			},
		})
		return
	}

	// Convert to OpenAI format
	openaiModels := []translator.Model{}
	for _, model := range models {
		openaiModels = append(openaiModels, translator.Model{
			ID:      model.ID,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: model.Provider,
		})
	}

	c.JSON(http.StatusOK, translator.ModelsResponse{
		Object: "list",
		Data:   openaiModels,
	})
}

// GetModel handles GET /v1/models/{model}
func (h *OpenAIHandler) GetModel(c *gin.Context) {
	modelID := c.Param("model")

	modelInfo, err := h.router.GetModelInfo(c.Request.Context(), modelID)
	if err != nil {
		c.JSON(http.StatusNotFound, translator.ErrorResponse{
			Error: translator.ErrorDetail{
				Message: fmt.Sprintf("Model %q not found", modelID),
				Type:    "invalid_request_error",
				Code:    "model_not_found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, translator.Model{
		ID:      modelInfo.ID,
		Object:  "model",
		Created: time.Now().Unix(),
		OwnedBy: modelInfo.Provider,
	})
}
