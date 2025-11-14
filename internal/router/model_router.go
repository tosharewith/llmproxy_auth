package router

import (
	"fmt"
	"strings"

	"github.com/tosharewith/llmproxy_auth/internal/providers"
)

// ModelRouter routes models to their appropriate providers
type ModelRouter struct {
	providers map[string]providers.Provider
	modelMap  map[string]string // model -> provider name mapping
}

// NewModelRouter creates a new model router
func NewModelRouter() *ModelRouter {
	return &ModelRouter{
		providers: make(map[string]providers.Provider),
		modelMap:  make(map[string]string),
	}
}

// RegisterProvider registers a provider with the router
func (r *ModelRouter) RegisterProvider(provider providers.Provider) error {
	name := provider.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider already registered: %s", name)
	}

	r.providers[name] = provider
	return nil
}

// RegisterModelMapping registers a model-to-provider mapping
func (r *ModelRouter) RegisterModelMapping(model, providerName string) error {
	if _, exists := r.providers[providerName]; !exists {
		return fmt.Errorf("provider not found: %s", providerName)
	}

	r.modelMap[model] = providerName
	return nil
}

// RouteModel routes a model to its provider
func (r *ModelRouter) RouteModel(model string) (providers.Provider, error) {
	// Try exact match first
	if providerName, ok := r.modelMap[model]; ok {
		return r.providers[providerName], nil
	}

	// Try pattern matching
	providerName := r.matchModelPattern(model)
	if providerName != "" {
		if provider, ok := r.providers[providerName]; ok {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("no provider found for model: %s", model)
}

// matchModelPattern matches a model to a provider using patterns
func (r *ModelRouter) matchModelPattern(model string) string {
	// Check suffixes first (these take priority over prefixes)

	// Azure OpenAI (deployment-based naming)
	if strings.HasSuffix(model, "-azure") || strings.HasSuffix(model, "-deployment") {
		return "azure"
	}

	// Anthropic Direct API
	if strings.HasSuffix(model, "-anthropic") {
		return "anthropic"
	}

	// IBM watsonx.ai
	if strings.HasPrefix(model, "ibm/") {
		return "ibm"
	}

	// Oracle Cloud AI (Cohere on OCI) - check before Bedrock
	if strings.HasPrefix(model, "cohere.") && !strings.Contains(model, "command-text") {
		// cohere.command-r-plus → Oracle
		// cohere.command-text → Bedrock
		return "oracle"
	}

	// AWS Bedrock models
	bedrockPrefixes := []string{
		"claude-",
		"amazon.titan-",
		"ai21.j2-",
		"meta.llama",
		"mistral.",
		"cohere.command-text", // Bedrock-specific Cohere
	}
	for _, prefix := range bedrockPrefixes {
		if strings.HasPrefix(model, prefix) {
			return "bedrock"
		}
	}

	// OpenAI models
	openaiPrefixes := []string{
		"gpt-3.5-",
		"gpt-4",
		"text-davinci-",
		"text-curie-",
		"text-babbage-",
		"text-ada-",
	}
	for _, prefix := range openaiPrefixes {
		if strings.HasPrefix(model, prefix) {
			return "openai"
		}
	}

	// Google Vertex AI models
	vertexPrefixes := []string{
		"gemini-",
		"text-bison",
		"chat-bison",
		"codechat-bison",
	}
	for _, prefix := range vertexPrefixes {
		if strings.HasPrefix(model, prefix) {
			return "vertex"
		}
	}

	return ""
}

// GetProvider returns a provider by name
func (r *ModelRouter) GetProvider(name string) (providers.Provider, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return provider, nil
}

// ListProviders returns all registered providers
func (r *ModelRouter) ListProviders() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// GetProviderForModel returns the provider name for a model
func (r *ModelRouter) GetProviderForModel(model string) string {
	// Check exact match
	if providerName, ok := r.modelMap[model]; ok {
		return providerName
	}

	// Check pattern match
	return r.matchModelPattern(model)
}
