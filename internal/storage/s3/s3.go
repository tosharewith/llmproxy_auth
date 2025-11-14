// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/tosharewith/llmproxy_auth/internal/storage"
)

// S3Provider implements the StorageProvider interface for AWS S3
type S3Provider struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	region        string
}

// Config for S3 provider
type S3Config struct {
	Region string
}

// NewS3Provider creates a new S3 storage provider
func NewS3Provider(cfg S3Config) (*S3Provider, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1" // Default region
	}

	// Load AWS config with default credential chain (IRSA, instance profile, env vars)
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg)

	// Create presign client
	presignClient := s3.NewPresignClient(client)

	return &S3Provider{
		client:        client,
		presignClient: presignClient,
		region:        cfg.Region,
	}, nil
}

// Name returns the provider name
func (p *S3Provider) Name() string {
	return "s3"
}

// GetObject retrieves an object from S3
func (p *S3Provider) GetObject(ctx context.Context, req *storage.GetObjectRequest) (*storage.GetObjectResponse, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Key),
	}

	// Add range header if specified
	if req.RangeStart != nil || req.RangeEnd != nil {
		start := int64(0)
		if req.RangeStart != nil {
			start = *req.RangeStart
		}

		rangeStr := fmt.Sprintf("bytes=%d-", start)
		if req.RangeEnd != nil {
			rangeStr = fmt.Sprintf("bytes=%d-%d", start, *req.RangeEnd)
		}
		input.Range = aws.String(rangeStr)
	}

	result, err := p.client.GetObject(ctx, input)
	if err != nil {
		return nil, p.handleError("GetObject", err)
	}

	// Extract metadata
	metadata := make(map[string]string)
	for k, v := range result.Metadata {
		metadata[k] = v
	}

	return &storage.GetObjectResponse{
		Body:          result.Body,
		ContentType:   aws.ToString(result.ContentType),
		ContentLength: aws.ToInt64(result.ContentLength),
		LastModified:  aws.ToTime(result.LastModified),
		ETag:          aws.ToString(result.ETag),
		Metadata:      metadata,
	}, nil
}

// PutObject uploads an object to S3
func (p *S3Provider) PutObject(ctx context.Context, req *storage.PutObjectRequest) (*storage.PutObjectResponse, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(req.Bucket),
		Key:         aws.String(req.Key),
		Body:        req.Body,
		ContentType: aws.String(req.ContentType),
	}

	// Add metadata
	if len(req.Metadata) > 0 {
		input.Metadata = req.Metadata
	}

	// Configure server-side encryption
	if req.SSE != nil {
		switch req.SSE.Algorithm {
		case "AES256":
			input.ServerSideEncryption = types.ServerSideEncryptionAes256
		case "aws:kms":
			input.ServerSideEncryption = types.ServerSideEncryptionAwsKms
			if req.SSE.KMSKeyID != "" {
				input.SSEKMSKeyId = aws.String(req.SSE.KMSKeyID)
			}
		}
	}

	result, err := p.client.PutObject(ctx, input)
	if err != nil {
		return nil, p.handleError("PutObject", err)
	}

	return &storage.PutObjectResponse{
		ETag:      aws.ToString(result.ETag),
		VersionID: aws.ToString(result.VersionId),
	}, nil
}

// DeleteObject removes an object from S3
func (p *S3Provider) DeleteObject(ctx context.Context, req *storage.DeleteObjectRequest) (*storage.DeleteObjectResponse, error) {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Key),
	}

	if req.VersionID != "" {
		input.VersionId = aws.String(req.VersionID)
	}

	result, err := p.client.DeleteObject(ctx, input)
	if err != nil {
		return nil, p.handleError("DeleteObject", err)
	}

	return &storage.DeleteObjectResponse{
		DeleteMarker: aws.ToBool(result.DeleteMarker),
		VersionID:    aws.ToString(result.VersionId),
	}, nil
}

// ListObjects lists objects in an S3 bucket
func (p *S3Provider) ListObjects(ctx context.Context, req *storage.ListObjectsRequest) (*storage.ListObjectsResponse, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(req.Bucket),
	}

	if req.Prefix != "" {
		input.Prefix = aws.String(req.Prefix)
	}

	if req.Delimiter != "" {
		input.Delimiter = aws.String(req.Delimiter)
	}

	if req.MaxKeys > 0 {
		input.MaxKeys = aws.Int32(int32(req.MaxKeys))
	}

	if req.StartAfter != "" {
		input.StartAfter = aws.String(req.StartAfter)
	}

	if req.ContinuationToken != "" {
		input.ContinuationToken = aws.String(req.ContinuationToken)
	}

	result, err := p.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, p.handleError("ListObjects", err)
	}

	// Convert objects
	objects := make([]storage.ObjectInfo, 0, len(result.Contents))
	for _, obj := range result.Contents {
		objects = append(objects, storage.ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
			ETag:         aws.ToString(obj.ETag),
			StorageClass: string(obj.StorageClass),
		})
	}

	// Convert common prefixes (directories)
	commonPrefixes := make([]string, 0, len(result.CommonPrefixes))
	for _, prefix := range result.CommonPrefixes {
		commonPrefixes = append(commonPrefixes, aws.ToString(prefix.Prefix))
	}

	return &storage.ListObjectsResponse{
		Objects:               objects,
		CommonPrefixes:        commonPrefixes,
		IsTruncated:           aws.ToBool(result.IsTruncated),
		NextContinuationToken: aws.ToString(result.NextContinuationToken),
	}, nil
}

// HeadObject gets object metadata without downloading
func (p *S3Provider) HeadObject(ctx context.Context, req *storage.HeadObjectRequest) (*storage.HeadObjectResponse, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Key),
	}

	result, err := p.client.HeadObject(ctx, input)
	if err != nil {
		return nil, p.handleError("HeadObject", err)
	}

	// Extract metadata
	metadata := make(map[string]string)
	for k, v := range result.Metadata {
		metadata[k] = v
	}

	return &storage.HeadObjectResponse{
		ContentType:   aws.ToString(result.ContentType),
		ContentLength: aws.ToInt64(result.ContentLength),
		LastModified:  aws.ToTime(result.LastModified),
		ETag:          aws.ToString(result.ETag),
		Metadata:      metadata,
		StorageClass:  string(result.StorageClass),
	}, nil
}

// GeneratePresignedURL generates a presigned URL for temporary access
func (p *S3Provider) GeneratePresignedURL(ctx context.Context, req *storage.PresignRequest) (*storage.PresignedURL, error) {
	expiresAt := time.Now().Add(req.ExpiresIn)

	var presignedURL *v4.PresignedHTTPRequest
	var err error

	switch req.Operation {
	case storage.PresignOperationGet:
		presignedURL, err = p.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(req.Bucket),
			Key:    aws.String(req.Key),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = req.ExpiresIn
		})

	case storage.PresignOperationPut:
		input := &s3.PutObjectInput{
			Bucket: aws.String(req.Bucket),
			Key:    aws.String(req.Key),
		}
		if req.ContentType != "" {
			input.ContentType = aws.String(req.ContentType)
		}
		presignedURL, err = p.presignClient.PresignPutObject(ctx, input, func(opts *s3.PresignOptions) {
			opts.Expires = req.ExpiresIn
		})

	case storage.PresignOperationDelete:
		presignedURL, err = p.presignClient.PresignDeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(req.Bucket),
			Key:    aws.String(req.Key),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = req.ExpiresIn
		})

	case storage.PresignOperationHead:
		presignedURL, err = p.presignClient.PresignHeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(req.Bucket),
			Key:    aws.String(req.Key),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = req.ExpiresIn
		})

	default:
		return nil, &storage.StorageError{
			Provider:   "s3",
			Operation:  "GeneratePresignedURL",
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("unsupported presign operation: %s", req.Operation),
		}
	}

	if err != nil {
		return nil, p.handleError("GeneratePresignedURL", err)
	}

	return &storage.PresignedURL{
		URL:       presignedURL.URL,
		ExpiresIn: int(req.ExpiresIn.Seconds()),
		ExpiresAt: expiresAt.Format(time.RFC3339),
		Operation: req.Operation,
		Bucket:    req.Bucket,
		Key:       req.Key,
	}, nil
}

// HealthCheck verifies S3 is accessible
func (p *S3Provider) HealthCheck(ctx context.Context) error {
	// Simple health check - list buckets
	_, err := p.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("S3 health check failed: %w", err)
	}
	return nil
}

// handleError converts AWS S3 errors to StorageError
func (p *S3Provider) handleError(operation string, err error) error {
	storageErr := &storage.StorageError{
		Provider:   "s3",
		Operation:  operation,
		StatusCode: http.StatusInternalServerError,
		Message:    "S3 operation failed",
		Err:        err,
	}

	// Map specific S3 errors to appropriate status codes
	errStr := err.Error()

	if contains(errStr, "NoSuchKey") || contains(errStr, "NotFound") {
		storageErr.StatusCode = http.StatusNotFound
		storageErr.Message = "Object not found"
	} else if contains(errStr, "NoSuchBucket") {
		storageErr.StatusCode = http.StatusNotFound
		storageErr.Message = "Bucket not found"
	} else if contains(errStr, "AccessDenied") || contains(errStr, "Forbidden") {
		storageErr.StatusCode = http.StatusForbidden
		storageErr.Message = "Access denied"
	} else if contains(errStr, "InvalidRequest") || contains(errStr, "BadRequest") {
		storageErr.StatusCode = http.StatusBadRequest
		storageErr.Message = "Invalid request"
	}

	return storageErr
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
