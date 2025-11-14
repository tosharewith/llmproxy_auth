// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tosharewith/llmproxy_auth/internal/storage"
)

// StorageHandler handles cloud storage requests
type StorageHandler struct {
	providers     map[string]storage.StorageProvider
	accessControl *StorageAccessControl
}

// NewStorageHandler creates a new storage handler
func NewStorageHandler(providers map[string]storage.StorageProvider, ac *StorageAccessControl) *StorageHandler {
	if ac == nil {
		ac = NewDefaultAccessControl()
	}

	return &StorageHandler{
		providers:     providers,
		accessControl: ac,
	}
}

// Handle processes storage requests
// Path format: /-{provider}/{env}/{operation}/{bucket}/{key...}
// Example: /-s3/prod/presign/rag-docs/quantum.md?ttl=3600
func (h *StorageHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Parse path components
	// Remove leading /-
	path := strings.TrimPrefix(r.URL.Path, "/-")
	parts := strings.SplitN(path, "/", 5)

	if len(parts) < 4 {
		h.writeError(w, http.StatusBadRequest, "Invalid storage path format")
		return
	}

	providerName := parts[0]  // s3, azure, gcs
	_ = parts[1]              // environment (prod, dev, staging) - reserved for future use
	operation := parts[2]     // get, put, delete, list, presign, head
	bucketAndKey := parts[3:] // bucket and optional key

	// Get provider
	provider, ok := h.providers[providerName]
	if !ok {
		h.writeError(w, http.StatusNotFound, fmt.Sprintf("Storage provider %q not found", providerName))
		return
	}

	// Parse bucket and key
	bucket := ""
	key := ""
	if len(bucketAndKey) > 0 {
		bucket = bucketAndKey[0]
	}
	if len(bucketAndKey) > 1 {
		key = strings.Join(bucketAndKey[1:], "/")
	}

	// Check access control
	if !h.accessControl.CheckAccess(r, bucket, key, operation) {
		h.writeError(w, http.StatusForbidden, "Access denied")
		return
	}

	// Route to appropriate operation
	ctx := r.Context()

	switch operation {
	case "get":
		if key == "" {
			h.writeError(w, http.StatusBadRequest, "Object key is required for get operation")
			return
		}

		resp, err := provider.GetObject(ctx, &storage.GetObjectRequest{
			Bucket: bucket,
			Key:    key,
		})
		if err != nil {
			h.handleStorageError(w, err)
			return
		}
		defer resp.Body.Close()

		// Set response headers
		w.Header().Set("Content-Type", resp.ContentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
		w.Header().Set("ETag", resp.ETag)
		w.Header().Set("Last-Modified", resp.LastModified.Format(http.TimeFormat))

		// Stream body to client
		w.WriteHeader(http.StatusOK)
		io.Copy(w, resp.Body)

	case "put":
		if key == "" {
			h.writeError(w, http.StatusBadRequest, "Object key is required for put operation")
			return
		}

		contentType := r.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		resp, err := provider.PutObject(ctx, &storage.PutObjectRequest{
			Bucket:      bucket,
			Key:         key,
			Body:        r.Body,
			ContentType: contentType,
		})
		if err != nil {
			h.handleStorageError(w, err)
			return
		}

		// Write success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"etag":       resp.ETag,
			"version_id": resp.VersionID,
		})

	case "delete":
		if key == "" {
			h.writeError(w, http.StatusBadRequest, "Object key is required for delete operation")
			return
		}

		_, err := provider.DeleteObject(ctx, &storage.DeleteObjectRequest{
			Bucket: bucket,
			Key:    key,
		})
		if err != nil {
			h.handleStorageError(w, err)
			return
		}

		// Write success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})

	case "list":
		prefix := r.URL.Query().Get("prefix")
		delimiter := r.URL.Query().Get("delimiter")
		maxKeysStr := r.URL.Query().Get("max_keys")
		continuationToken := r.URL.Query().Get("continuation_token")

		maxKeys := 1000 // default
		if maxKeysStr != "" {
			if parsed, err := strconv.Atoi(maxKeysStr); err == nil {
				maxKeys = parsed
			}
		}

		resp, err := provider.ListObjects(ctx, &storage.ListObjectsRequest{
			Bucket:            bucket,
			Prefix:            prefix,
			Delimiter:         delimiter,
			MaxKeys:           maxKeys,
			ContinuationToken: continuationToken,
		})
		if err != nil {
			h.handleStorageError(w, err)
			return
		}

		// Write response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"objects":                  resp.Objects,
			"common_prefixes":          resp.CommonPrefixes,
			"is_truncated":             resp.IsTruncated,
			"next_continuation_token":  resp.NextContinuationToken,
		})

	case "head":
		if key == "" {
			h.writeError(w, http.StatusBadRequest, "Object key is required for head operation")
			return
		}

		resp, err := provider.HeadObject(ctx, &storage.HeadObjectRequest{
			Bucket: bucket,
			Key:    key,
		})
		if err != nil {
			h.handleStorageError(w, err)
			return
		}

		// Write response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content_type":   resp.ContentType,
			"content_length": resp.ContentLength,
			"last_modified":  resp.LastModified.Format(time.RFC3339),
			"etag":           resp.ETag,
			"metadata":       resp.Metadata,
			"storage_class":  resp.StorageClass,
		})

	case "presign":
		if key == "" {
			h.writeError(w, http.StatusBadRequest, "Object key is required for presign operation")
			return
		}

		// Parse TTL from query parameter
		ttlStr := r.URL.Query().Get("ttl")
		if ttlStr == "" {
			ttlStr = "3600" // Default: 1 hour
		}

		ttlSeconds, err := strconv.Atoi(ttlStr)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "Invalid TTL value")
			return
		}

		// Parse operation (default: GetObject)
		presignOp := storage.PresignOperationGet
		opStr := r.URL.Query().Get("operation")
		if opStr != "" {
			presignOp = storage.PresignOperation(opStr)
		}

		// Generate presigned URL
		resp, err := provider.GeneratePresignedURL(ctx, &storage.PresignRequest{
			Bucket:    bucket,
			Key:       key,
			Operation: presignOp,
			ExpiresIn: time.Duration(ttlSeconds) * time.Second,
		})
		if err != nil {
			h.handleStorageError(w, err)
			return
		}

		// Write response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)

	default:
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("Unknown storage operation: %s", operation))
	}
}

// handleStorageError converts storage errors to HTTP responses
func (h *StorageHandler) handleStorageError(w http.ResponseWriter, err error) {
	if storageErr, ok := err.(*storage.StorageError); ok {
		h.writeError(w, storageErr.StatusCode, storageErr.Message)
		return
	}

	h.writeError(w, http.StatusInternalServerError, "Storage operation failed")
}

// writeError writes an error response
func (h *StorageHandler) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"code":    statusCode,
		},
	})
}

// StorageAccessControl manages access control for storage operations
type StorageAccessControl struct {
	AllowedBuckets   []string
	DeniedPrefixes   []string
	AllowedProviders []string
}

// NewDefaultAccessControl creates a default access control (permissive)
func NewDefaultAccessControl() *StorageAccessControl {
	return &StorageAccessControl{
		AllowedBuckets:   []string{}, // Empty = allow all
		DeniedPrefixes:   []string{"/secret/", "/private/", "/."},
		AllowedProviders: []string{"s3", "azure", "gcs"},
	}
}

// CheckAccess validates access to a storage operation
func (ac *StorageAccessControl) CheckAccess(r *http.Request, bucket, key, operation string) bool {
	// Check bucket allowlist (if configured)
	if len(ac.AllowedBuckets) > 0 {
		bucketAllowed := false
		for _, allowed := range ac.AllowedBuckets {
			if bucket == allowed {
				bucketAllowed = true
				break
			}
		}
		if !bucketAllowed {
			return false
		}
	}

	// Check key against denied prefixes
	for _, denied := range ac.DeniedPrefixes {
		if strings.HasPrefix("/"+key, denied) || strings.HasPrefix(key, denied) {
			return false
		}
	}

	return true
}
