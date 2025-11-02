# Parameter Mapping Documentation

This document provides comprehensive documentation for how request parameters are translated between different AI provider APIs.

---

## Overview

When using protocol mode (e.g., OpenAI-compatible API), the gateway translates request parameters from the source protocol format to the target provider format, and response parameters back to the protocol format.

Different providers support different parameters, so not all parameters can be translated. This document details:
- ✅ **Supported parameters** - Translated automatically
- ⚠️ **Partially supported** - Translated with limitations
- ❌ **Unsupported parameters** - Ignored (not translated)
- 🔧 **Provider-specific** - Available via `additionalModelRequestFields`

---

## OpenAI → AWS Bedrock Converse API

### Request Parameter Mapping

| OpenAI Parameter | Bedrock Parameter | Status | Notes |
|-----------------|-------------------|--------|-------|
| `model` | Model ID mapping | ✅ Supported | Mapped via model registry |
| `messages` | `messages` | ✅ Supported | Full content type support (text, images) |
| `max_tokens` | `inferenceConfig.maxTokens` | ✅ Supported | Direct mapping |
| `temperature` | `inferenceConfig.temperature` | ✅ Supported | Range: 0.0-1.0 (both) |
| `top_p` | `inferenceConfig.topP` | ✅ Supported | Range: 0.0-1.0 (both) |
| `stop` | `inferenceConfig.stopSequences` | ✅ Supported | Array of strings |
| `stream` | Converse-stream endpoint | ✅ Supported | Uses different endpoint |
| `tools` | `toolConfig.tools` | ✅ Supported | Function calling |
| `tool_choice` | `toolConfig.toolChoice` | ✅ Supported | auto, any, specific tool |
| `n` | - | ❌ Not supported | Bedrock doesn't support multiple completions |
| `frequency_penalty` | - | ❌ Not supported | Bedrock doesn't have this parameter |
| `presence_penalty` | - | ❌ Not supported | Bedrock doesn't have this parameter |
| `logit_bias` | - | ❌ Not supported | Bedrock doesn't expose logit control |
| `user` | - | ❌ Not supported | Bedrock doesn't track user IDs |
| `seed` | - | ❌ Not supported | Bedrock doesn't support deterministic output |
| `response_format` | - | ❌ Not supported | Bedrock doesn't enforce JSON mode |
| - | `additionalModelRequestFields` | 🔧 Provider-specific | For model-specific parameters |

### Response Parameter Mapping

| Bedrock Parameter | OpenAI Parameter | Status | Notes |
|------------------|------------------|--------|-------|
| `output.message.content` | `choices[0].message.content` | ✅ Supported | Text content |
| `output.message.content[].toolUse` | `choices[0].message.tool_calls` | ✅ Supported | Function calls |
| `stopReason` | `choices[0].finish_reason` | ✅ Supported | Mapped (see table below) |
| `usage.inputTokens` | `usage.prompt_tokens` | ✅ Supported | Direct mapping |
| `usage.outputTokens` | `usage.completion_tokens` | ✅ Supported | Direct mapping |
| `usage.totalTokens` | `usage.total_tokens` | ✅ Supported | Direct mapping |
| `metrics.latencyMs` | - | ❌ Not exposed | Could be added as custom field |

### Stop Reason Mapping

| Bedrock `stopReason` | OpenAI `finish_reason` |
|---------------------|------------------------|
| `end_turn` | `stop` |
| `max_tokens` | `length` |
| `stop_sequence` | `stop` |
| `tool_use` | `tool_calls` |
| `content_filtered` | `content_filter` |

---

## Model-Specific Parameters (Bedrock)

Different Bedrock models support different parameters via `additionalModelRequestFields`:

### Claude Models (Anthropic)

| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `top_k` | integer | Top-K sampling | `250` |
| `anthropic_version` | string | API version | `"bedrock-2023-05-31"` |

**Usage** (not currently supported in gateway):
```json
{
  "model": "claude-3-sonnet",
  "messages": [...],
  "additionalModelRequestFields": {
    "top_k": 250
  }
}
```

### Titan Models (Amazon)

| Parameter | Type | Description |
|-----------|------|-------------|
| `textGenerationConfig` | object | Text generation settings |

### Meta Llama Models

| Parameter | Type | Description |
|-----------|------|-------------|
| `max_gen_len` | integer | Maximum generation length |

### AI21 Jurassic Models

| Parameter | Type | Description |
|-----------|------|-------------|
| `maxTokens` | integer | Maximum tokens |
| `temperature` | float | Temperature |
| `topP` | float | Top-P sampling |
| `countPenalty` | object | Repetition penalty |

---

## OpenAI → OpenAI (Passthrough)

When using OpenAI provider with OpenAI protocol, all parameters are passed through without translation:

| Parameter | Status | Notes |
|-----------|--------|-------|
| `model` | ✅ Passed through | Native OpenAI model names |
| `messages` | ✅ Passed through | Full OpenAI format |
| `max_tokens` | ✅ Passed through | |
| `temperature` | ✅ Passed through | Range: 0.0-2.0 |
| `top_p` | ✅ Passed through | |
| `n` | ✅ Passed through | Multiple completions supported |
| `stream` | ✅ Passed through | |
| `stop` | ✅ Passed through | |
| `frequency_penalty` | ✅ Passed through | Range: -2.0 to 2.0 |
| `presence_penalty` | ✅ Passed through | Range: -2.0 to 2.0 |
| `logit_bias` | ✅ Passed through | Token ID mapping |
| `user` | ✅ Passed through | User tracking |
| `seed` | ✅ Passed through | Deterministic output |
| `response_format` | ✅ Passed through | JSON mode |
| `tools` | ✅ Passed through | Function calling |
| `tool_choice` | ✅ Passed through | |

---

## OpenAI → Anthropic Messages API

### Request Parameter Mapping

| OpenAI Parameter | Anthropic Parameter | Status | Notes |
|-----------------|---------------------|--------|-------|
| `model` | `model` | ✅ Supported | Mapped to Claude model IDs |
| `messages` | `messages` | ✅ Supported | Content format differs |
| `max_tokens` | `max_tokens` | ✅ Supported | **Required** in Anthropic API |
| `temperature` | `temperature` | ✅ Supported | Range: 0.0-1.0 |
| `top_p` | `top_p` | ✅ Supported | Range: 0.0-1.0 |
| `stop` | `stop_sequences` | ✅ Supported | Array of strings |
| `stream` | `stream` | ✅ Supported | SSE format |
| `tools` | `tools` | ✅ Supported | Function calling |
| `tool_choice` | `tool_choice` | ✅ Supported | auto, any, tool |
| - | `top_k` | 🔧 Anthropic-specific | Not in OpenAI API |
| `n` | - | ❌ Not supported | Anthropic doesn't support multiple completions |
| `frequency_penalty` | - | ❌ Not supported | |
| `presence_penalty` | - | ❌ Not supported | |
| `logit_bias` | - | ❌ Not supported | |
| `user` | - | ❌ Not supported | |
| `seed` | - | ❌ Not supported | |

**Important**: Anthropic **requires** `max_tokens` to be set. The gateway should provide a default if not specified.

---

## OpenAI → Google Vertex AI (Gemini)

### Request Parameter Mapping

| OpenAI Parameter | Vertex Parameter | Status | Notes |
|-----------------|------------------|--------|-------|
| `model` | `model` | ✅ Supported | Mapped to Gemini model names |
| `messages` | `contents` | ✅ Supported | Role mapping: `assistant` → `model` |
| `max_tokens` | `generationConfig.maxOutputTokens` | ✅ Supported | |
| `temperature` | `generationConfig.temperature` | ✅ Supported | Range: 0.0-2.0 |
| `top_p` | `generationConfig.topP` | ✅ Supported | |
| `stop` | `generationConfig.stopSequences` | ✅ Supported | |
| `stream` | `streamGenerateContent` | ✅ Supported | Different endpoint |
| `tools` | `tools` | ✅ Supported | Function declarations |
| - | `generationConfig.topK` | 🔧 Vertex-specific | Top-K sampling |
| - | `generationConfig.candidateCount` | 🔧 Vertex-specific | Multiple candidates |
| `n` | - | ⚠️ Partial | Maps to `candidateCount` but different semantics |
| `frequency_penalty` | - | ❌ Not supported | |
| `presence_penalty` | - | ❌ Not supported | |

**Role Mapping**:
- OpenAI `assistant` → Vertex `model`
- OpenAI `user` → Vertex `user`
- OpenAI `system` → Vertex system instruction (different structure)

---

## OpenAI → IBM watsonx.ai

### Request Parameter Mapping

| OpenAI Parameter | IBM Parameter | Status | Notes |
|-----------------|---------------|--------|-------|
| `model` | `model_id` | ✅ Supported | Mapped to IBM model IDs |
| `messages` | `input` (text) | ⚠️ Partial | Converted to single text prompt |
| `max_tokens` | `parameters.max_new_tokens` | ✅ Supported | |
| `temperature` | `parameters.temperature` | ✅ Supported | |
| `top_p` | `parameters.top_p` | ✅ Supported | |
| `stop` | `parameters.stop_sequences` | ✅ Supported | |
| - | `parameters.top_k` | 🔧 IBM-specific | Top-K sampling |
| - | `parameters.repetition_penalty` | 🔧 IBM-specific | Repetition control |
| `n` | - | ❌ Not supported | |
| `stream` | - | ❌ Not supported | IBM uses different streaming API |
| `tools` | - | ❌ Not supported | IBM doesn't support function calling |

---

## OpenAI → Oracle Cloud AI (Cohere)

### Request Parameter Mapping

| OpenAI Parameter | Oracle Parameter | Status | Notes |
|-----------------|------------------|--------|-------|
| `model` | `modelId` | ✅ Supported | Mapped to Cohere model IDs |
| `messages` | `prompt` | ⚠️ Partial | Converted to single prompt |
| `max_tokens` | `maxTokens` | ✅ Supported | |
| `temperature` | `temperature` | ✅ Supported | |
| `top_p` | `topP` | ✅ Supported | |
| `stop` | `stopSequences` | ✅ Supported | |
| - | `topK` | 🔧 Cohere-specific | Top-K sampling |
| - | `frequencyPenalty` | 🔧 Cohere-specific | Different from OpenAI |
| - | `presencePenalty` | 🔧 Cohere-specific | Different from OpenAI |
| `n` | `numGenerations` | ✅ Supported | Multiple completions |
| `stream` | - | ❌ Not supported | Different streaming format |
| `tools` | - | ❌ Not supported | |

---

## Azure OpenAI → Azure OpenAI (Passthrough)

All parameters pass through unchanged as Azure uses OpenAI format.

---

## Current Implementation Status

### ✅ Fully Implemented
- OpenAI → OpenAI (passthrough)
- Azure → Azure (passthrough)
- Basic parameter translation for Bedrock (max_tokens, temperature, top_p, stop)
- Message translation (text content)
- Tool/function calling (OpenAI ↔ Bedrock)

### ⚠️ Partially Implemented
- OpenAI → Bedrock (missing frequency_penalty, presence_penalty, n, seed, etc.)
- Multi-modal content (images supported but not documents)

### ❌ Not Implemented
- OpenAI → Anthropic direct translation
- OpenAI → Vertex AI translation
- OpenAI → IBM translation
- OpenAI → Oracle translation
- Provider-specific parameters via `additionalModelRequestFields`
- Multiple completions (n > 1)
- Response format enforcement (JSON mode)
- Logit bias
- Seed/deterministic output

---

## Missing Parameters and Workarounds

### Frequency Penalty & Presence Penalty

**OpenAI**: Controls repetition through penalties (-2.0 to 2.0)
**Bedrock**: Not supported in Converse API

**Workaround**: None available. These parameters are silently ignored.

### Top-K Sampling

**Anthropic/Vertex/IBM**: Support top-k sampling
**OpenAI**: Does not expose top-k parameter
**Bedrock**: Supports via `additionalModelRequestFields` for Claude models

**Workaround**: Could be exposed via custom parameter extension:
```json
{
  "model": "claude-3-sonnet",
  "messages": [...],
  "extra_params": {
    "top_k": 250
  }
}
```

### Multiple Completions (n > 1)

**OpenAI**: Supports generating multiple completions
**Bedrock**: Does not support multiple completions
**Anthropic**: Does not support multiple completions
**Vertex**: Supports via `candidateCount` (different semantics)

**Workaround**: Make multiple sequential requests (not recommended for cost/latency).

### Seed (Deterministic Output)

**OpenAI**: Supports seed for reproducible outputs
**Others**: Not supported

**Workaround**: None available.

### Response Format (JSON Mode)

**OpenAI**: Supports `response_format: {type: "json_object"}`
**Others**: Not supported consistently

**Workaround**: Use prompt engineering ("respond in JSON format").

---

## Recommendations for Enhancement

### Priority 1: High Impact
1. ✅ **Document all parameter mappings** (this document)
2. 🔧 **Add missing common parameters**:
   - `frequency_penalty` → Best effort mapping or ignore with warning
   - `presence_penalty` → Best effort mapping or ignore with warning
   - `n` → Error if n > 1 for providers that don't support it
   - `seed` → Ignore with warning

### Priority 2: Provider Parity
3. 🔧 **Support provider-specific parameters**:
   - Add `additionalModelRequestFields` support for Bedrock
   - Add `extra_params` extension for provider-specific features
   - Document which providers support which extra parameters

4. 🔧 **Add warnings for unsupported parameters**:
   - Log warnings when parameters are ignored
   - Return warnings in response metadata
   - Add validation mode (strict vs permissive)

### Priority 3: Advanced Features
5. 🔧 **Add parameter validation**:
   - Validate parameter ranges per provider
   - Validate required parameters (e.g., Anthropic requires max_tokens)
   - Return clear errors for invalid parameters

6. 🔧 **Add parameter defaults**:
   - Provide sensible defaults per provider
   - Make defaults configurable in YAML
   - Document default behavior

---

## Configuration Examples

### Setting Default Parameters per Instance

```yaml
bedrock_us1_openai:
  type: bedrock
  mode: protocol
  protocol: openai
  region: us-east-1

  transformation:
    request_from: openai
    request_to: bedrock_converse
    response_from: bedrock_converse
    response_to: openai

    options:
      # Default parameters
      default_max_tokens: 4096
      default_temperature: 0.7
      default_top_p: 0.9

      # Parameter handling
      preserve_original_model_id: false
      warn_on_unsupported_params: true
      strict_parameter_validation: false

      # Provider-specific parameters
      allow_additional_model_fields: true
      additional_fields_mapping:
        top_k: "top_k"  # Map custom top_k to Bedrock's top_k
```

---

## Testing Parameter Translation

Test script for verifying parameter translation:

```bash
# Test with all common parameters
curl -X POST http://localhost:8090/openai/bedrock_us1_openai/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "claude-3-sonnet",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 100,
    "temperature": 0.7,
    "top_p": 0.9,
    "stop": ["Human:", "Assistant:"],
    "frequency_penalty": 0.5,
    "presence_penalty": 0.5,
    "n": 1
  }'
```

**Expected behavior**:
- ✅ max_tokens, temperature, top_p, stop → Translated
- ⚠️ frequency_penalty, presence_penalty → Ignored (should warn)
- ✅ n=1 → OK (n>1 should error)

---

## See Also

- [Transparent and Protocol Modes](./TRANSPARENT-AND-PROTOCOL-MODES.md)
- [AWS Bedrock Converse API Documentation](https://docs.aws.amazon.com/bedrock/latest/userguide/conversation-inference.html)
- [OpenAI API Reference](https://platform.openai.com/docs/api-reference/chat)
- [Anthropic Messages API](https://docs.anthropic.com/claude/reference/messages_post)
- [Vertex AI Gemini API](https://cloud.google.com/vertex-ai/docs/generative-ai/model-reference/gemini)
