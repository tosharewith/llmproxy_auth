// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package bedrock

import "github.com/tosharewith/llmproxy_auth/internal/providers"

// BedrockModels defines all available Bedrock models
var BedrockModels = []providers.Model{
	// Claude 3 family
	{
		ID:            "claude-3-opus",
		Provider:      "bedrock",
		Name:          "Claude 3 Opus",
		Description:   "Most capable Claude model for complex tasks",
		Capabilities:  []string{providers.CapabilityChat, providers.CapabilityStreaming, providers.CapabilityVision},
		ContextWindow: 200000,
		InputPrice:    15.00,  // $15 per 1M input tokens
		OutputPrice:   75.00,  // $75 per 1M output tokens
		Available:     true,
	},
	{
		ID:            "claude-3-sonnet",
		Provider:      "bedrock",
		Name:          "Claude 3 Sonnet",
		Description:   "Balanced performance and speed for most tasks",
		Capabilities:  []string{providers.CapabilityChat, providers.CapabilityStreaming, providers.CapabilityVision},
		ContextWindow: 200000,
		InputPrice:    3.00,   // $3 per 1M input tokens
		OutputPrice:   15.00,  // $15 per 1M output tokens
		Available:     true,
	},
	{
		ID:            "claude-3-haiku",
		Provider:      "bedrock",
		Name:          "Claude 3 Haiku",
		Description:   "Fastest Claude model for simple tasks",
		Capabilities:  []string{providers.CapabilityChat, providers.CapabilityStreaming, providers.CapabilityVision},
		ContextWindow: 200000,
		InputPrice:    0.25,   // $0.25 per 1M input tokens
		OutputPrice:   1.25,   // $1.25 per 1M output tokens
		Available:     true,
	},
	{
		ID:            "claude-3-5-sonnet",
		Provider:      "bedrock",
		Name:          "Claude 3.5 Sonnet",
		Description:   "Latest Claude model with enhanced capabilities",
		Capabilities:  []string{providers.CapabilityChat, providers.CapabilityStreaming, providers.CapabilityVision},
		ContextWindow: 200000,
		InputPrice:    3.00,   // $3 per 1M input tokens
		OutputPrice:   15.00,  // $15 per 1M output tokens
		Available:     true,
	},

	// Amazon Titan family
	{
		ID:            "amazon-titan-text-express",
		Provider:      "bedrock",
		Name:          "Titan Text Express",
		Description:   "Amazon's text generation model optimized for speed",
		Capabilities:  []string{providers.CapabilityChat, providers.CapabilityCompletion},
		ContextWindow: 8192,
		InputPrice:    0.20,   // $0.20 per 1M input tokens
		OutputPrice:   0.60,   // $0.60 per 1M output tokens
		Available:     true,
	},
	{
		ID:            "amazon-titan-text-lite",
		Provider:      "bedrock",
		Name:          "Titan Text Lite",
		Description:   "Lightweight Amazon text model for simple tasks",
		Capabilities:  []string{providers.CapabilityChat, providers.CapabilityCompletion},
		ContextWindow: 4096,
		InputPrice:    0.15,   // $0.15 per 1M input tokens
		OutputPrice:   0.20,   // $0.20 per 1M output tokens
		Available:     true,
	},
	{
		ID:            "amazon-titan-embed-text",
		Provider:      "bedrock",
		Name:          "Titan Embeddings",
		Description:   "Amazon's text embedding model",
		Capabilities:  []string{providers.CapabilityEmbeddings},
		ContextWindow: 8192,
		InputPrice:    0.10,   // $0.10 per 1M input tokens
		OutputPrice:   0.00,   // No output tokens for embeddings
		Available:     true,
	},

	// Meta Llama family
	{
		ID:            "llama2-13b",
		Provider:      "bedrock",
		Name:          "Llama 2 13B Chat",
		Description:   "Meta's Llama 2 13B parameter chat model",
		Capabilities:  []string{providers.CapabilityChat, providers.CapabilityCompletion},
		ContextWindow: 4096,
		InputPrice:    0.75,   // $0.75 per 1M input tokens
		OutputPrice:   1.00,   // $1.00 per 1M output tokens
		Available:     true,
	},
	{
		ID:            "llama2-70b",
		Provider:      "bedrock",
		Name:          "Llama 2 70B Chat",
		Description:   "Meta's Llama 2 70B parameter chat model",
		Capabilities:  []string{providers.CapabilityChat, providers.CapabilityCompletion},
		ContextWindow: 4096,
		InputPrice:    1.95,   // $1.95 per 1M input tokens
		OutputPrice:   2.56,   // $2.56 per 1M output tokens
		Available:     true,
	},

	// Mistral family
	{
		ID:            "mistral-7b",
		Provider:      "bedrock",
		Name:          "Mistral 7B Instruct",
		Description:   "Mistral's 7B parameter instruction-tuned model",
		Capabilities:  []string{providers.CapabilityChat, providers.CapabilityCompletion},
		ContextWindow: 32768,
		InputPrice:    0.15,   // $0.15 per 1M input tokens
		OutputPrice:   0.20,   // $0.20 per 1M output tokens
		Available:     true,
	},
	{
		ID:            "mistral-8x7b",
		Provider:      "bedrock",
		Name:          "Mixtral 8x7B",
		Description:   "Mistral's mixture-of-experts model",
		Capabilities:  []string{providers.CapabilityChat, providers.CapabilityCompletion},
		ContextWindow: 32768,
		InputPrice:    0.45,   // $0.45 per 1M input tokens
		OutputPrice:   0.70,   // $0.70 per 1M output tokens
		Available:     true,
	},
}

// GetBedrockModelInfo returns model information for a given model ID
func GetBedrockModelInfo(modelID string) *providers.Model {
	for i := range BedrockModels {
		if BedrockModels[i].ID == modelID {
			return &BedrockModels[i]
		}
	}
	return nil
}

// BedrockModelIDMap maps friendly names to Bedrock model IDs
var BedrockModelIDMap = map[string]string{
	// Claude 3 family
	"claude-3-opus":                "anthropic.claude-3-opus-20240229-v1:0",
	"claude-3-opus-20240229":       "anthropic.claude-3-opus-20240229-v1:0",
	"claude-3-sonnet":              "anthropic.claude-3-sonnet-20240229-v1:0",
	"claude-3-sonnet-20240229":     "anthropic.claude-3-sonnet-20240229-v1:0",
	"claude-3-haiku":               "anthropic.claude-3-haiku-20240307-v1:0",
	"claude-3-haiku-20240307":      "anthropic.claude-3-haiku-20240307-v1:0",
	"claude-3-5-sonnet":            "anthropic.claude-3-5-sonnet-20240620-v1:0",
	"claude-3-5-sonnet-20240620":   "anthropic.claude-3-5-sonnet-20240620-v1:0",

	// Amazon Titan
	"amazon-titan-text-express":    "amazon.titan-text-express-v1",
	"amazon-titan-text-lite":       "amazon.titan-text-lite-v1",
	"amazon-titan-embed-text":      "amazon.titan-embed-text-v1",

	// Meta Llama
	"llama2-13b":                   "meta.llama2-13b-chat-v1",
	"llama2-70b":                   "meta.llama2-70b-chat-v1",

	// Mistral
	"mistral-7b":                   "mistral.mistral-7b-instruct-v0:2",
	"mistral-8x7b":                 "mistral.mixtral-8x7b-instruct-v0:1",
}

// GetBedrockModelID returns the full Bedrock model ID for a friendly name
func GetBedrockModelID(friendlyName string) (string, bool) {
	// Check if it's already a full Bedrock model ID
	if len(friendlyName) > 0 && (friendlyName[0:1] == "anthropic." ||
		friendlyName[0:1] == "amazon." ||
		friendlyName[0:1] == "meta." ||
		friendlyName[0:1] == "mistral.") {
		return friendlyName, true
	}

	// Look up in map
	modelID, exists := BedrockModelIDMap[friendlyName]
	return modelID, exists
}
