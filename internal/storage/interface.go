// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"io"
	"time"
)

// StorageProvider defines the interface for cloud storage providers
type StorageProvider interface {
	// Name returns the provider name (s3, azure, gcs)
	Name() string

	// GetObject retrieves an object from storage
	GetObject(ctx context.Context, req *GetObjectRequest) (*GetObjectResponse, error)

	// PutObject uploads an object to storage
	PutObject(ctx context.Context, req *PutObjectRequest) (*PutObjectResponse, error)

	// DeleteObject removes an object from storage
	DeleteObject(ctx context.Context, req *DeleteObjectRequest) (*DeleteObjectResponse, error)

	// ListObjects lists objects in a bucket/container
	ListObjects(ctx context.Context, req *ListObjectsRequest) (*ListObjectsResponse, error)

	// GeneratePresignedURL generates a presigned URL for temporary access
	GeneratePresignedURL(ctx context.Context, req *PresignRequest) (*PresignedURL, error)

	// HeadObject gets object metadata without downloading
	HeadObject(ctx context.Context, req *HeadObjectRequest) (*HeadObjectResponse, error)

	// HealthCheck verifies the provider is accessible
	HealthCheck(ctx context.Context) error
}

// GetObjectRequest represents a request to download an object
type GetObjectRequest struct {
	Bucket string
	Key    string
	// Optional: byte range for partial downloads
	RangeStart *int64
	RangeEnd   *int64
}

// GetObjectResponse represents the response from GetObject
type GetObjectResponse struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
	LastModified  time.Time
	ETag          string
	Metadata      map[string]string
}

// PutObjectRequest represents a request to upload an object
type PutObjectRequest struct {
	Bucket      string
	Key         string
	Body        io.Reader
	ContentType string
	Metadata    map[string]string
	// Optional: Server-side encryption
	SSE *ServerSideEncryption
}

// ServerSideEncryption configures server-side encryption
type ServerSideEncryption struct {
	Algorithm string // AES256, aws:kms, etc.
	KMSKeyID  string // Optional: KMS key ID
}

// PutObjectResponse represents the response from PutObject
type PutObjectResponse struct {
	ETag         string
	VersionID    string
	StorageClass string
}

// DeleteObjectRequest represents a request to delete an object
type DeleteObjectRequest struct {
	Bucket    string
	Key       string
	VersionID string // Optional: specific version to delete
}

// DeleteObjectResponse represents the response from DeleteObject
type DeleteObjectResponse struct {
	DeleteMarker bool
	VersionID    string
}

// ListObjectsRequest represents a request to list objects
type ListObjectsRequest struct {
	Bucket       string
	Prefix       string
	Delimiter    string
	MaxKeys      int
	StartAfter   string // For pagination
	ContinuationToken string // For pagination
}

// ListObjectsResponse represents the response from ListObjects
type ListObjectsResponse struct {
	Objects              []ObjectInfo
	CommonPrefixes       []string
	IsTruncated          bool
	NextContinuationToken string
}

// ObjectInfo contains metadata about a storage object
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	StorageClass string
}

// HeadObjectRequest represents a request to get object metadata
type HeadObjectRequest struct {
	Bucket string
	Key    string
}

// HeadObjectResponse represents the response from HeadObject
type HeadObjectResponse struct {
	ContentType   string
	ContentLength int64
	LastModified  time.Time
	ETag          string
	Metadata      map[string]string
	StorageClass  string
}

// PresignRequest represents a request to generate a presigned URL
type PresignRequest struct {
	Bucket    string
	Key       string
	Operation PresignOperation
	ExpiresIn time.Duration // TTL for the presigned URL
	// Optional: Content-Type for PutObject presigned URLs
	ContentType string
}

// PresignOperation defines the allowed operation for a presigned URL
type PresignOperation string

const (
	PresignOperationGet    PresignOperation = "GetObject"
	PresignOperationPut    PresignOperation = "PutObject"
	PresignOperationDelete PresignOperation = "DeleteObject"
	PresignOperationHead   PresignOperation = "HeadObject"
)

// PresignedURL represents a presigned URL response
type PresignedURL struct {
	URL       string           `json:"url"`
	ExpiresIn int              `json:"expires_in"` // Seconds until expiration
	ExpiresAt string           `json:"expires_at"` // RFC3339 timestamp
	Operation PresignOperation `json:"operation"`
	Bucket    string           `json:"bucket"`
	Key       string           `json:"key"`
}

// StorageError represents a storage provider error
type StorageError struct {
	Provider   string
	Operation  string
	StatusCode int
	Message    string
	Err        error
}

func (e *StorageError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *StorageError) Unwrap() error {
	return e.Err
}

// Common error codes
const (
	ErrCodeNotFound        = "NotFound"
	ErrCodeAccessDenied    = "AccessDenied"
	ErrCodeInvalidRequest  = "InvalidRequest"
	ErrCodeBucketNotFound  = "BucketNotFound"
	ErrCodeObjectTooLarge  = "ObjectTooLarge"
	ErrCodeInternalError   = "InternalError"
)
