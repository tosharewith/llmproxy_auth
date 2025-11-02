// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tosharewith/llmproxy_auth/internal/providers"
	"github.com/tosharewith/llmproxy_auth/internal/providers/bedrock"
)

// BedrockRequest represents a Bedrock-specific request format
type BedrockRequest struct {
	AnthropicVersion string                   `json:"anthropic_version,omitempty"`
	Messages         []BedrockMessage         `json:"messages"`
	MaxTokens        int                      `json:"max_tokens,omitempty"`
	Temperature      float64                  `json:"temperature,omitempty"`
	TopP             float64                  `json:"top_p,omitempty"`
	TopK             int                      `json:"top_k,omitempty"`
	StopSequences    []string                 `json:"stop_sequences,omitempty"`
	System           string                   `json:"system,omitempty"`
}

// BedrockMessage represents a message in Bedrock format
type BedrockMessage struct {
	Role    string                 `json:"role"` // user or assistant
	Content interface{}            `json:"content"` // string or []BedrockContentBlock
}

// BedrockContentBlock represents a content block (for multimodal)
type BedrockContentBlock struct {
	Type   string            `json:"type"` // text or image
	Text   string            `json:"text,omitempty"`
	Source *BedrockImageSource `json:"source,omitempty"`
}

// BedrockImageSource represents an image source
type BedrockImageSource struct {
	Type      string `json:"type"` // base64
	MediaType string `json:"media_type"` // image/jpeg, image/png, etc.
	Data      string `json:"data"` // base64 encoded image
}

// BedrockResponse represents a Bedrock response
type BedrockResponse struct {
	ID           string                `json:"id"`
	Type         string                `json:"type"` // message
	Role         string                `json:"role"` // assistant
	Content      []BedrockContentBlock `json:"content"`
	Model        string                `json:"model"`
	StopReason   string                `json:"stop_reason"` // end_turn, max_tokens, stop_sequence
	StopSequence string                `json:"stop_sequence,omitempty"`
	Usage        BedrockUsage          `json:"usage"`
}

// BedrockUsage represents token usage
type BedrockUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// TranslateOpenAIToBedrock converts an OpenAI chat completion request to Bedrock format
func TranslateOpenAIToBedrock(openaiReq *ChatCompletionRequest) (*providers.ProviderRequest, string, error) {
	// Get the Bedrock model ID
	bedrockModelID, exists := bedrock.GetBedrockModelID(openaiReq.Model)
	if !exists {
		return nil, "", fmt.Errorf("model %q not supported on Bedrock", openaiReq.Model)
	}

	// Convert messages
	bedrockMessages := []BedrockMessage{}
	var systemPrompt string

	for _, msg := range openaiReq.Messages {
		// Handle system messages separately for Claude
		if msg.Role == "system" {
			systemPrompt = extractTextContent(msg.Content)
			continue
		}

		// Skip assistant messages with tool calls (not yet supported)
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			continue
		}

		// Convert message
		bedrockMsg := BedrockMessage{
			Role: msg.Role,
		}

		// Handle content
		switch content := msg.Content.(type) {
		case string:
			bedrockMsg.Content = content
		case []interface{}:
			// Multimodal content
			blocks := []BedrockContentBlock{}
			for _, part := range content {
				if partMap, ok := part.(map[string]interface{}); ok {
					block := convertContentPart(partMap)
					if block != nil {
						blocks = append(blocks, *block)
					}
				}
			}
			if len(blocks) > 0 {
				bedrockMsg.Content = blocks
			}
		default:
			bedrockMsg.Content = fmt.Sprintf("%v", content)
		}

		bedrockMessages = append(bedrockMessages, bedrockMsg)
	}

	// Build Bedrock request
	bedrockReq := BedrockRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		Messages:         bedrockMessages,
		MaxTokens:        openaiReq.MaxTokens,
		Temperature:      openaiReq.Temperature,
		TopP:             openaiReq.TopP,
		StopSequences:    openaiReq.Stop,
		System:           systemPrompt,
	}

	// Set defaults
	if bedrockReq.MaxTokens == 0 {
		bedrockReq.MaxTokens = 4096
	}
	if bedrockReq.Temperature == 0 {
		bedrockReq.Temperature = 1.0
	}

	// Marshal to JSON
	body, err := json.Marshal(bedrockReq)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal Bedrock request: %w", err)
	}

	// Build provider request
	path := fmt.Sprintf("/model/%s/invoke", bedrockModelID)
	if openaiReq.Stream {
		path = fmt.Sprintf("/model/%s/invoke-with-response-stream", bedrockModelID)
	}

	providerReq := &providers.ProviderRequest{
		Method: "POST",
		Path:   path,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		Body: body,
	}

	return providerReq, bedrockModelID, nil
}

// TranslateBedrockToOpenAI converts a Bedrock response to OpenAI format
func TranslateBedrockToOpenAI(bedrockResp *BedrockResponse, openaiModel string, requestID string) *ChatCompletionResponse {
	// Extract text content from response
	var content string
	for _, block := range bedrockResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	// Map stop reason
	finishReason := mapStopReason(bedrockResp.StopReason)

	// Build OpenAI response
	return &ChatCompletionResponse{
		ID:      requestID,
		Object:  "chat.completion",
		Created: currentTimestamp(),
		Model:   openaiModel,
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
		Usage: &Usage{
			PromptTokens:     bedrockResp.Usage.InputTokens,
			CompletionTokens: bedrockResp.Usage.OutputTokens,
			TotalTokens:      bedrockResp.Usage.InputTokens + bedrockResp.Usage.OutputTokens,
		},
	}
}

// extractTextContent extracts text from content (handles string or array)
func extractTextContent(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var texts []string
		for _, part := range c {
			if partMap, ok := part.(map[string]interface{}); ok {
				if text, ok := partMap["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
		return strings.Join(texts, "\n")
	default:
		return fmt.Sprintf("%v", content)
	}
}

// convertContentPart converts an OpenAI content part to Bedrock format
func convertContentPart(part map[string]interface{}) *BedrockContentBlock {
	partType, ok := part["type"].(string)
	if !ok {
		return nil
	}

	switch partType {
	case "text":
		if text, ok := part["text"].(string); ok {
			return &BedrockContentBlock{
				Type: "text",
				Text: text,
			}
		}

	case "image_url":
		if imageURL, ok := part["image_url"].(map[string]interface{}); ok {
			if url, ok := imageURL["url"].(string); ok {
				// Extract base64 data from data URL
				if strings.HasPrefix(url, "data:image/") {
					parts := strings.SplitN(url, ",", 2)
					if len(parts) == 2 {
						mediaType := extractMediaType(parts[0])
						return &BedrockContentBlock{
							Type: "image",
							Source: &BedrockImageSource{
								Type:      "base64",
								MediaType: mediaType,
								Data:      parts[1],
							},
						}
					}
				}
			}
		}
	}

	return nil
}

// extractMediaType extracts media type from data URL prefix
func extractMediaType(prefix string) string {
	// prefix format: "data:image/jpeg;base64"
	parts := strings.Split(prefix, ";")
	if len(parts) > 0 {
		mediaType := strings.TrimPrefix(parts[0], "data:")
		return mediaType
	}
	return "image/jpeg" // default
}

// mapStopReason maps Bedrock stop reason to OpenAI finish reason
func mapStopReason(bedrockReason string) string {
	switch bedrockReason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}

// currentTimestamp returns current Unix timestamp
func currentTimestamp() int64 {
	return 0 // TODO: implement proper timestamp
}
