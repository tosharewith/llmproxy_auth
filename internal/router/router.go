// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package router

import (
	"context"
	"fmt"
	"log"

	"github.com/tosharewith/llmproxy_auth/internal/providers"
)

// Router handles routing requests to appropriate providers
type Router struct {
	config    *Config
	providers map[string]providers.Provider
}

// NewRouter creates a new router with the given configuration
func NewRouter(config *Config, providerRegistry map[string]providers.Provider) (*Router, error) {
	// Validate configuration
	if err := config.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &Router{
		config:    config,
		providers: providerRegistry,
	}, nil
}

// RouteRequest determines which provider should handle a request
func (r *Router) RouteRequest(ctx context.Context, modelName string, preferredProvider string) (providers.Provider, *ProviderModelInfo, error) {
	// If preferred provider is specified and valid, use it
	if preferredProvider != "" {
		if provider, modelInfo, err := r.getProviderForModel(modelName, preferredProvider); err == nil {
			return provider, modelInfo, nil
		}
		log.Printf("Preferred provider %q not available for model %q, falling back to default", preferredProvider, modelName)
	}

	// Get default provider for the model
	defaultProvider := r.config.GetDefaultProvider(modelName)
	if defaultProvider == "" {
		return nil, nil, fmt.Errorf("no provider found for model %q", modelName)
	}

	// Try default provider
	provider, modelInfo, err := r.getProviderForModel(modelName, defaultProvider)
	if err == nil {
		return provider, modelInfo, nil
	}

	// If auto-fallback is disabled, return the error
	if !r.config.Features.AutoFallback || !r.config.Routing.Fallback.Enabled {
		return nil, nil, fmt.Errorf("provider %q failed for model %q: %w", defaultProvider, modelName, err)
	}

	// Try fallback providers
	log.Printf("Default provider %q failed for model %q, attempting fallback", defaultProvider, modelName)
	return r.tryFallbackProviders(ctx, modelName, defaultProvider)
}

// getProviderForModel gets a specific provider for a model
func (r *Router) getProviderForModel(modelName, providerName string) (providers.Provider, *ProviderModelInfo, error) {
	// Check if provider is enabled
	if !r.config.IsProviderEnabled(providerName) {
		return nil, nil, fmt.Errorf("provider %q is disabled", providerName)
	}

	// Get provider instance
	provider, exists := r.providers[providerName]
	if !exists {
		return nil, nil, fmt.Errorf("provider %q not registered", providerName)
	}

	// Get model info for this provider
	modelInfo, err := r.config.GetProviderModelInfo(modelName, providerName)
	if err != nil {
		return nil, nil, fmt.Errorf("model %q not available on provider %q: %w", modelName, providerName, err)
	}

	return provider, modelInfo, nil
}

// tryFallbackProviders attempts to find an alternative provider
func (r *Router) tryFallbackProviders(ctx context.Context, modelName, excludeProvider string) (providers.Provider, *ProviderModelInfo, error) {
	fallbackProviders := r.config.GetFallbackProviders()
	attempts := 0
	maxAttempts := r.config.Routing.Fallback.MaxAttempts

	for _, providerName := range fallbackProviders {
		// Skip the failed provider
		if providerName == excludeProvider {
			continue
		}

		// Check attempt limit
		if attempts >= maxAttempts {
			break
		}
		attempts++

		// Try this fallback provider
		provider, modelInfo, err := r.getProviderForModel(modelName, providerName)
		if err == nil {
			log.Printf("Successfully failed over to provider %q for model %q", providerName, modelName)
			return provider, modelInfo, nil
		}

		log.Printf("Fallback provider %q also failed for model %q: %v", providerName, modelName, err)
	}

	return nil, nil, fmt.Errorf("all fallback providers exhausted for model %q", modelName)
}

// GetProvider gets a provider by name
func (r *Router) GetProvider(providerName string) (providers.Provider, error) {
	if !r.config.IsProviderEnabled(providerName) {
		return nil, fmt.Errorf("provider %q is disabled", providerName)
	}

	provider, exists := r.providers[providerName]
	if !exists {
		return nil, fmt.Errorf("provider %q not registered", providerName)
	}

	return provider, nil
}

// ListModels lists all available models across all enabled providers
func (r *Router) ListModels(ctx context.Context) ([]providers.Model, error) {
	var allModels []providers.Model

	// Get models from configuration
	for modelName, mapping := range r.config.ModelMappings {
		// Only include models whose default provider is enabled
		if !r.config.IsProviderEnabled(mapping.DefaultProvider) {
			continue
		}

		// Get provider
		provider, exists := r.providers[mapping.DefaultProvider]
		if !exists {
			continue
		}

		// Try to get model info from provider
		modelInfo, err := provider.GetModelInfo(ctx, modelName)
		if err != nil {
			// If provider doesn't have the model info, create a basic entry
			allModels = append(allModels, providers.Model{
				ID:       modelName,
				Provider: mapping.DefaultProvider,
				Name:     modelName,
				Available: true,
			})
			continue
		}

		allModels = append(allModels, *modelInfo)
	}

	return allModels, nil
}

// GetModelInfo gets information about a specific model
func (r *Router) GetModelInfo(ctx context.Context, modelName string) (*providers.Model, error) {
	// Get default provider for the model
	defaultProvider := r.config.GetDefaultProvider(modelName)
	if defaultProvider == "" {
		return nil, fmt.Errorf("model %q not found", modelName)
	}

	// Get provider
	provider, exists := r.providers[defaultProvider]
	if !exists {
		return nil, fmt.Errorf("provider %q not available", defaultProvider)
	}

	// Get model info
	return provider.GetModelInfo(ctx, modelName)
}

// HealthCheck performs health checks on all enabled providers
func (r *Router) HealthCheck(ctx context.Context) map[string]error {
	results := make(map[string]error)

	for name, provider := range r.providers {
		if !r.config.IsProviderEnabled(name) {
			continue
		}

		err := provider.HealthCheck(ctx)
		results[name] = err
	}

	return results
}

// GetConfig returns the router configuration
func (r *Router) GetConfig() *Config {
	return r.config
}

// RegisterProvider registers a new provider (useful for testing)
func (r *Router) RegisterProvider(name string, provider providers.Provider) {
	r.providers[name] = provider
}

// UnregisterProvider removes a provider (useful for testing)
func (r *Router) UnregisterProvider(name string) {
	delete(r.providers, name)
}
