package router

import (
	"testing"
)

// TestMultiProviderRouting tests routing different models to their providers
func TestMultiProviderRouting(t *testing.T) {
	// Create router
	router := NewModelRouter()

	tests := []struct {
		name             string
		model            string
		expectedProvider string
	}{
		// AWS Bedrock models
		{
			name:             "Claude 3 Sonnet → Bedrock",
			model:            "claude-3-sonnet-20240229",
			expectedProvider: "bedrock",
		},
		{
			name:             "Claude 3 Opus → Bedrock",
			model:            "claude-3-opus-20240229",
			expectedProvider: "bedrock",
		},
		{
			name:             "Claude 3.5 Sonnet → Bedrock",
			model:            "claude-3-5-sonnet-20240620",
			expectedProvider: "bedrock",
		},
		{
			name:             "Amazon Titan → Bedrock",
			model:            "amazon.titan-text-express-v1",
			expectedProvider: "bedrock",
		},

		// OpenAI models
		{
			name:             "GPT-4 → OpenAI",
			model:            "gpt-4",
			expectedProvider: "openai",
		},
		{
			name:             "GPT-4 Turbo → OpenAI",
			model:            "gpt-4-turbo",
			expectedProvider: "openai",
		},
		{
			name:             "GPT-3.5 Turbo → OpenAI",
			model:            "gpt-3.5-turbo",
			expectedProvider: "openai",
		},

		// Azure OpenAI models
		{
			name:             "GPT-4 deployment on Azure → Azure",
			model:            "gpt-4-azure-deployment",
			expectedProvider: "azure",
		},

		// Google Vertex AI models
		{
			name:             "Gemini Pro → Vertex AI",
			model:            "gemini-pro",
			expectedProvider: "vertex",
		},
		{
			name:             "Gemini 1.5 Pro → Vertex AI",
			model:            "gemini-1.5-pro",
			expectedProvider: "vertex",
		},

		// Anthropic Direct
		{
			name:             "Claude via Anthropic API → Anthropic",
			model:            "claude-3-sonnet-20240229-anthropic",
			expectedProvider: "anthropic",
		},

		// IBM watsonx.ai
		{
			name:             "Granite model → IBM watsonx",
			model:            "ibm/granite-13b-chat-v2",
			expectedProvider: "ibm",
		},

		// Oracle Cloud AI
		{
			name:             "Cohere model on OCI → Oracle",
			model:            "cohere.command-r-plus",
			expectedProvider: "oracle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerName := router.GetProviderForModel(tt.model)

			if providerName != tt.expectedProvider {
				t.Errorf("provider: got %q, want %q", providerName, tt.expectedProvider)
			}
		})
	}
}

// TestModelRouterRegistration tests provider registration
func TestModelRouterRegistration(t *testing.T) {
	router := NewModelRouter()

	// Test that initially there are no providers
	providers := router.ListProviders()
	if len(providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(providers))
	}

	// Note: Actual provider registration will be tested with real provider implementations
}

// Capability constants for testing
type ProviderCapabilities struct {
	SupportsStreaming bool
	SupportsVision    bool
	SupportsTools     bool
	MaxTokens         int
}

func getExpectedCapabilities(provider string) ProviderCapabilities {
	// Expected capabilities for each provider
	capabilities := map[string]ProviderCapabilities{
		"bedrock": {
			SupportsStreaming: true,
			SupportsVision:    true,
			SupportsTools:     true,
			MaxTokens:         200000,
		},
		"openai": {
			SupportsStreaming: true,
			SupportsVision:    true,
			SupportsTools:     true,
			MaxTokens:         128000,
		},
		"anthropic": {
			SupportsStreaming: true,
			SupportsVision:    true,
			SupportsTools:     true,
			MaxTokens:         200000,
		},
		"vertex": {
			SupportsStreaming: true,
			SupportsVision:    true,
			SupportsTools:     true,
			MaxTokens:         32000,
		},
		"azure": {
			SupportsStreaming: true,
			SupportsVision:    true,
			SupportsTools:     true,
			MaxTokens:         128000,
		},
		"ibm": {
			SupportsStreaming: false,
			SupportsVision:    false,
			SupportsTools:     false,
			MaxTokens:         8192,
		},
		"oracle": {
			SupportsStreaming: true,
			SupportsVision:    false,
			SupportsTools:     true,
			MaxTokens:         4096,
		},
	}

	if caps, ok := capabilities[provider]; ok {
		return caps
	}

	// Default capabilities
	return ProviderCapabilities{
		SupportsStreaming: false,
		SupportsVision:    false,
		SupportsTools:     false,
		MaxTokens:         4096,
	}
}

// TestProviderCapabilities tests expected provider capabilities
func TestProviderCapabilities(t *testing.T) {
	tests := []struct {
		provider string
		check    func(caps ProviderCapabilities) bool
		desc     string
	}{
		{
			provider: "bedrock",
			check: func(caps ProviderCapabilities) bool {
				return caps.SupportsStreaming && caps.SupportsVision &&
					caps.SupportsTools && caps.MaxTokens == 200000
			},
			desc: "Bedrock should support streaming, vision, tools with 200k tokens",
		},
		{
			provider: "openai",
			check: func(caps ProviderCapabilities) bool {
				return caps.SupportsStreaming && caps.SupportsVision &&
					caps.SupportsTools && caps.MaxTokens == 128000
			},
			desc: "OpenAI should support streaming, vision, tools with 128k tokens",
		},
		{
			provider: "ibm",
			check: func(caps ProviderCapabilities) bool {
				return !caps.SupportsStreaming && !caps.SupportsVision &&
					!caps.SupportsTools && caps.MaxTokens == 8192
			},
			desc: "IBM should not support streaming/vision/tools, 8k tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			caps := getExpectedCapabilities(tt.provider)
			if !tt.check(caps) {
				t.Errorf("%s: capabilities check failed", tt.desc)
			}
		})
	}
}
