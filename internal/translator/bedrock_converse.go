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

// Bedrock Converse API types (unified API for all models)

// ConverseRequest represents a Bedrock Converse API request
type ConverseRequest struct {
	Messages         []ConverseMessage         `json:"messages"`
	System           []SystemContentBlock      `json:"system,omitempty"`
	InferenceConfig  *InferenceConfig          `json:"inferenceConfig,omitempty"`
	ToolConfig       *ToolConfig               `json:"toolConfig,omitempty"`
	AdditionalModelRequestFields map[string]interface{} `json:"additionalModelRequestFields,omitempty"`
}

// ConverseMessage represents a message in Converse API
type ConverseMessage struct {
	Role    string               `json:"role"` // user or assistant
	Content []ContentBlock       `json:"content"`
}

// ContentBlock represents content (text, image, etc.)
type ContentBlock struct {
	Text     *string      `json:"text,omitempty"`
	Image    *ImageBlock  `json:"image,omitempty"`
	Document *DocumentBlock `json:"document,omitempty"`
	ToolUse  *ToolUseBlock `json:"toolUse,omitempty"`
	ToolResult *ToolResultBlock `json:"toolResult,omitempty"`
}

// ImageBlock represents an image
type ImageBlock struct {
	Format string         `json:"format"` // png, jpeg, gif, webp
	Source ImageSource    `json:"source"`
}

// ImageSource represents image source
type ImageSource struct {
	Bytes string `json:"bytes,omitempty"` // base64 encoded
}

// DocumentBlock represents a document
type DocumentBlock struct {
	Format string         `json:"format"` // pdf, csv, doc, docx, xls, xlsx, html, txt, md
	Name   string         `json:"name"`
	Source DocumentSource `json:"source"`
}

// DocumentSource represents document source
type DocumentSource struct {
	Bytes string `json:"bytes,omitempty"` // base64 encoded
}

// ToolUseBlock represents tool use
type ToolUseBlock struct {
	ToolUseId string                 `json:"toolUseId"`
	Name      string                 `json:"name"`
	Input     map[string]interface{} `json:"input"`
}

// ToolResultBlock represents tool result
type ToolResultBlock struct {
	ToolUseId string         `json:"toolUseId"`
	Content   []ContentBlock `json:"content"`
	Status    string         `json:"status,omitempty"` // success or error
}

// SystemContentBlock represents system content
type SystemContentBlock struct {
	Text string `json:"text,omitempty"`
}

// InferenceConfig represents inference configuration
type InferenceConfig struct {
	MaxTokens    *int     `json:"maxTokens,omitempty"`
	Temperature  *float64 `json:"temperature,omitempty"`
	TopP         *float64 `json:"topP,omitempty"`
	StopSequences []string `json:"stopSequences,omitempty"`
}

// ToolConfig represents tool configuration
type ToolConfig struct {
	Tools       []ConverseTool `json:"tools"`
	ToolChoice  *ToolChoice    `json:"toolChoice,omitempty"`
}

// ConverseTool represents a tool definition in Converse API
type ConverseTool struct {
	ToolSpec *ToolSpec `json:"toolSpec"`
}

// ToolSpec represents tool specification
type ToolSpec struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema *ToolInputSchema       `json:"inputSchema"`
}

// ToolInputSchema represents the JSON schema for tool input
type ToolInputSchema struct {
	JSON map[string]interface{} `json:"json"`
}

// ToolChoice represents how the model should use tools
type ToolChoice struct {
	Auto *struct{} `json:"auto,omitempty"`
	Any  *struct{} `json:"any,omitempty"`
	Tool *ToolChoiceTool `json:"tool,omitempty"`
}

// ToolChoiceTool represents a specific tool choice
type ToolChoiceTool struct {
	Name string `json:"name"`
}

// ConverseResponse represents a Bedrock Converse API response
type ConverseResponse struct {
	Output       ConverseOutput       `json:"output"`
	StopReason   string               `json:"stopReason"` // end_turn, max_tokens, stop_sequence, tool_use
	Usage        ConverseUsage        `json:"usage"`
	Metrics      *ConverseMetrics     `json:"metrics,omitempty"`
}

// ConverseOutput represents the response output
type ConverseOutput struct {
	Message *ConverseMessage `json:"message,omitempty"`
}

// ConverseUsage represents token usage
type ConverseUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

// ConverseMetrics represents performance metrics
type ConverseMetrics struct {
	LatencyMs int64 `json:"latencyMs"`
}

// TranslateOpenAIToConverseAPI converts OpenAI format to Bedrock Converse API format
func TranslateOpenAIToConverseAPI(openaiReq *ChatCompletionRequest) (*providers.ProviderRequest, string, error) {
	// Get the Bedrock model ID
	bedrockModelID, exists := bedrock.GetBedrockModelID(openaiReq.Model)
	if !exists {
		return nil, "", fmt.Errorf("model %q not supported on Bedrock", openaiReq.Model)
	}

	// Convert messages
	converseMessages := []ConverseMessage{}
	var systemBlocks []SystemContentBlock

	for _, msg := range openaiReq.Messages {
		// Handle system messages separately
		if msg.Role == "system" {
			systemBlocks = append(systemBlocks, SystemContentBlock{
				Text: extractTextContent(msg.Content),
			})
			continue
		}

		// Skip function/tool messages for now (can be added later)
		if msg.Role == "function" || msg.Role == "tool" {
			continue
		}

		// Convert message content
		contentBlocks := convertToContentBlocks(msg.Content)
		if len(contentBlocks) == 0 {
			continue
		}

		converseMessages = append(converseMessages, ConverseMessage{
			Role:    msg.Role,
			Content: contentBlocks,
		})
	}

	// Build inference config
	inferenceConfig := &InferenceConfig{}
	if openaiReq.MaxTokens > 0 {
		inferenceConfig.MaxTokens = &openaiReq.MaxTokens
	}
	if openaiReq.Temperature > 0 {
		inferenceConfig.Temperature = &openaiReq.Temperature
	}
	if openaiReq.TopP > 0 {
		inferenceConfig.TopP = &openaiReq.TopP
	}
	if len(openaiReq.Stop) > 0 {
		inferenceConfig.StopSequences = openaiReq.Stop
	}

	// Build tool config if functions or tools are provided
	var toolConfig *ToolConfig
	if len(openaiReq.Tools) > 0 || len(openaiReq.Functions) > 0 {
		toolConfig = convertToolsToConverseFormat(openaiReq)
	}

	// Build Converse request
	converseReq := ConverseRequest{
		Messages:        converseMessages,
		System:          systemBlocks,
		InferenceConfig: inferenceConfig,
		ToolConfig:      toolConfig,
	}

	// Marshal to JSON
	body, err := json.Marshal(converseReq)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal Converse request: %w", err)
	}

	// Build provider request
	path := fmt.Sprintf("/model/%s/converse", bedrockModelID)
	if openaiReq.Stream {
		path = fmt.Sprintf("/model/%s/converse-stream", bedrockModelID)
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

// TranslateConverseToOpenAI converts Bedrock Converse response to OpenAI format
func TranslateConverseToOpenAI(converseResp *ConverseResponse, openaiModel string, requestID string) *ChatCompletionResponse {
	// Extract content and tool calls from response
	var content string
	var toolCalls []ToolCall

	if converseResp.Output.Message != nil {
		for _, block := range converseResp.Output.Message.Content {
			if block.Text != nil {
				content += *block.Text
			}
			if block.ToolUse != nil {
				// Convert tool use to OpenAI format
				argsJSON, _ := json.Marshal(block.ToolUse.Input)
				toolCalls = append(toolCalls, ToolCall{
					ID:   block.ToolUse.ToolUseId,
					Type: "function",
					Function: FunctionCall{
						Name:      block.ToolUse.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}
	}

	// Map stop reason
	finishReason := mapConverseStopReason(converseResp.StopReason)

	// Build message
	message := ChatMessage{
		Role: "assistant",
	}

	if content != "" {
		message.Content = content
	}

	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	// Build OpenAI response
	return &ChatCompletionResponse{
		ID:      requestID,
		Object:  "chat.completion",
		Created: currentTimestampUnix(),
		Model:   openaiModel,
		Choices: []ChatCompletionChoice{
			{
				Index:        0,
				Message:      message,
				FinishReason: finishReason,
			},
		},
		Usage: &Usage{
			PromptTokens:     converseResp.Usage.InputTokens,
			CompletionTokens: converseResp.Usage.OutputTokens,
			TotalTokens:      converseResp.Usage.TotalTokens,
		},
	}
}

// convertToContentBlocks converts OpenAI content to Converse content blocks
func convertToContentBlocks(content interface{}) []ContentBlock {
	var blocks []ContentBlock

	switch c := content.(type) {
	case string:
		// Simple text content
		blocks = append(blocks, ContentBlock{
			Text: &c,
		})

	case []interface{}:
		// Multimodal content (array of content parts)
		for _, part := range c {
			if partMap, ok := part.(map[string]interface{}); ok {
				block := convertContentPartToBlock(partMap)
				if block != nil {
					blocks = append(blocks, *block)
				}
			}
		}

	default:
		// Fallback: convert to string
		text := fmt.Sprintf("%v", content)
		blocks = append(blocks, ContentBlock{
			Text: &text,
		})
	}

	return blocks
}

// convertContentPartToBlock converts an OpenAI content part to Converse content block
func convertContentPartToBlock(part map[string]interface{}) *ContentBlock {
	partType, ok := part["type"].(string)
	if !ok {
		return nil
	}

	switch partType {
	case "text":
		if text, ok := part["text"].(string); ok {
			return &ContentBlock{
				Text: &text,
			}
		}

	case "image_url":
		if imageURL, ok := part["image_url"].(map[string]interface{}); ok {
			if url, ok := imageURL["url"].(string); ok {
				// Extract base64 data from data URL
				if strings.HasPrefix(url, "data:image/") {
					parts := strings.SplitN(url, ",", 2)
					if len(parts) == 2 {
						format := extractImageFormat(parts[0])
						return &ContentBlock{
							Image: &ImageBlock{
								Format: format,
								Source: ImageSource{
									Bytes: parts[1],
								},
							},
						}
					}
				}
			}
		}
	}

	return nil
}

// extractImageFormat extracts image format from data URL prefix
func extractImageFormat(prefix string) string {
	// prefix format: "data:image/jpeg;base64"
	if strings.Contains(prefix, "image/png") {
		return "png"
	} else if strings.Contains(prefix, "image/jpeg") || strings.Contains(prefix, "image/jpg") {
		return "jpeg"
	} else if strings.Contains(prefix, "image/gif") {
		return "gif"
	} else if strings.Contains(prefix, "image/webp") {
		return "webp"
	}
	return "jpeg" // default
}

// mapConverseStopReason maps Converse stop reason to OpenAI finish reason
func mapConverseStopReason(converseReason string) string {
	switch converseReason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "content_filtered":
		return "content_filter"
	default:
		return "stop"
	}
}

// currentTimestampUnix returns current Unix timestamp
func currentTimestampUnix() int64 {
	return 0 // Will be set by handler
}

// convertToolsToConverseFormat converts OpenAI tools/functions to Converse format
func convertToolsToConverseFormat(req *ChatCompletionRequest) *ToolConfig {
	var converseTools []ConverseTool

	// Convert OpenAI tools (preferred format)
	for _, tool := range req.Tools {
		if tool.Type == "function" {
			converseTools = append(converseTools, ConverseTool{
				ToolSpec: &ToolSpec{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					InputSchema: &ToolInputSchema{
						JSON: tool.Function.Parameters,
					},
				},
			})
		}
	}

	// Convert legacy functions format
	for _, function := range req.Functions {
		converseTools = append(converseTools, ConverseTool{
			ToolSpec: &ToolSpec{
				Name:        function.Name,
				Description: function.Description,
				InputSchema: &ToolInputSchema{
					JSON: function.Parameters,
				},
			},
		})
	}

	if len(converseTools) == 0 {
		return nil
	}

	toolConfig := &ToolConfig{
		Tools: converseTools,
	}

	// Convert tool_choice
	if req.ToolChoice != nil {
		toolConfig.ToolChoice = convertToolChoice(req.ToolChoice)
	}

	return toolConfig
}

// convertToolChoice converts OpenAI tool_choice to Converse format
func convertToolChoice(toolChoice interface{}) *ToolChoice {
	switch tc := toolChoice.(type) {
	case string:
		switch tc {
		case "auto":
			return &ToolChoice{Auto: &struct{}{}}
		case "required", "any":
			return &ToolChoice{Any: &struct{}{}}
		case "none":
			return nil
		}
	case map[string]interface{}:
		if tcType, ok := tc["type"].(string); ok && tcType == "function" {
			if function, ok := tc["function"].(map[string]interface{}); ok {
				if name, ok := function["name"].(string); ok {
					return &ToolChoice{
						Tool: &ToolChoiceTool{Name: name},
					}
				}
			}
		}
	}
	return &ToolChoice{Auto: &struct{}{}} // Default to auto
}
