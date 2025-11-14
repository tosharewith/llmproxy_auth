// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// DocumentFetcher fetches and caches documents for RAG
type DocumentFetcher struct {
	httpClient *http.Client
	cache      *DocumentCache
}

// NewDocumentFetcher creates a new document fetcher
func NewDocumentFetcher(cacheTTL time.Duration) *DocumentFetcher {
	return &DocumentFetcher{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: NewDocumentCache(cacheTTL),
	}
}

// FetchDocument retrieves a document from a URL (typically a presigned URL)
func (f *DocumentFetcher) FetchDocument(ctx context.Context, url string) (*Document, error) {
	// Check cache first
	if doc := f.cache.Get(url); doc != nil {
		return doc, nil
	}

	// Fetch from URL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch document: HTTP %d", resp.StatusCode)
	}

	// Read document content
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read document: %w", err)
	}

	// Create document
	doc := &Document{
		URL:         url,
		Content:     content,
		ContentType: resp.Header.Get("Content-Type"),
		Size:        int64(len(content)),
		FetchedAt:   time.Now(),
	}

	// Compute content hash
	hash := sha256.Sum256(content)
	doc.ContentHash = hex.EncodeToString(hash[:])

	// Cache the document
	f.cache.Set(url, doc)

	return doc, nil
}

// Document represents a fetched document
type Document struct {
	URL         string
	Content     []byte
	ContentType string
	ContentHash string
	Size        int64
	FetchedAt   time.Time
}

// DocumentCache caches fetched documents
type DocumentCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

type cacheEntry struct {
	document  *Document
	expiresAt time.Time
}

// NewDocumentCache creates a new document cache
func NewDocumentCache(ttl time.Duration) *DocumentCache {
	cache := &DocumentCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a document from cache
func (c *DocumentCache) Get(url string) *Document {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[url]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return nil
	}

	return entry.document
}

// Set stores a document in cache
func (c *DocumentCache) Set(url string, doc *Document) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[url] = &cacheEntry{
		document:  doc,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes a document from cache
func (c *DocumentCache) Delete(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, url)
}

// Clear removes all documents from cache
func (c *DocumentCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
}

// Size returns the number of cached documents
func (c *DocumentCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// cleanupLoop periodically removes expired entries
func (c *DocumentCache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired entries
func (c *DocumentCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for url, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, url)
		}
	}
}
