package router

import (
	"testing"
)

// TestParseStoragePath tests detection of storage path prefixes
func TestParseStoragePath(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedType   string
		expectedRoute  string
		isStorage      bool
	}{
		{
			name:          "S3 storage path",
			path:          "/-s3/prod/presign/my-bucket/document.pdf",
			expectedType:  "s3",
			expectedRoute: "prod",
			isStorage:     true,
		},
		{
			name:          "Azure Blob storage path",
			path:          "/-azblob/dev/get/container/file.txt",
			expectedType:  "azblob",
			expectedRoute: "dev",
			isStorage:     true,
		},
		{
			name:          "GCP Blob storage path",
			path:          "/-gcpblob/staging/list/bucket/prefix/",
			expectedType:  "gcpblob",
			expectedRoute: "staging",
			isStorage:     true,
		},
		{
			name:          "IBM COS storage path",
			path:          "/-ibmcos/prod/get/bucket/key",
			expectedType:  "ibmcos",
			expectedRoute: "prod",
			isStorage:     true,
		},
		{
			name:          "AI provider path (OpenAI compatible)",
			path:          "/v1/chat/completions",
			expectedType:  "",
			expectedRoute: "",
			isStorage:     false,
		},
		{
			name:          "Transparent mode path",
			path:          "/transparent/bedrock/invoke-model",
			expectedType:  "",
			expectedRoute: "",
			isStorage:     false,
		},
		{
			name:          "HTTPS proxy path",
			path:          "/-https/prod/get/example.com/api/data",
			expectedType:  "https",
			expectedRoute: "prod",
			isStorage:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageType, route, isStorage := ParseStoragePath(tt.path)

			if isStorage != tt.isStorage {
				t.Errorf("isStorage: got %v, want %v", isStorage, tt.isStorage)
			}
			if storageType != tt.expectedType {
				t.Errorf("storageType: got %q, want %q", storageType, tt.expectedType)
			}
			if route != tt.expectedRoute {
				t.Errorf("route: got %q, want %q", route, tt.expectedRoute)
			}
		})
	}
}

// ParseStoragePath detects storage routing paths
func ParseStoragePath(path string) (storageType, route string, isStorage bool) {
	if len(path) < 3 || path[0] != '/' || path[1] != '-' {
		return "", "", false
	}

	// Find the storage type and route
	prefixes := map[string]string{
		"/-s3/":      "s3",
		"/-azblob/":  "azblob",
		"/-gcpblob/": "gcpblob",
		"/-ibmcos/":  "ibmcos",
		"/-ociobj/":  "ociobj",
		"/-https/":   "https",
	}

	for prefix, stype := range prefixes {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			// Extract route (next path segment)
			remaining := path[len(prefix):]
			for i, c := range remaining {
				if c == '/' {
					return stype, remaining[:i], true
				}
			}
			// No slash found, entire remaining is route
			return stype, remaining, true
		}
	}

	return "", "", false
}

// TestParseStorageOperation tests parsing of storage operations
func TestParseStorageOperation(t *testing.T) {
	tests := []struct {
		name              string
		path              string
		expectedOperation string
		expectedBucket    string
		expectedKey       string
	}{
		{
			name:              "S3 presign operation",
			path:              "/-s3/prod/presign/my-bucket/docs/file.pdf",
			expectedOperation: "presign",
			expectedBucket:    "my-bucket",
			expectedKey:       "docs/file.pdf",
		},
		{
			name:              "S3 get operation",
			path:              "/-s3/prod/get/data-bucket/reports/2024/jan.csv",
			expectedOperation: "get",
			expectedBucket:    "data-bucket",
			expectedKey:       "reports/2024/jan.csv",
		},
		{
			name:              "Azure Blob list operation",
			path:              "/-azblob/dev/list/container/prefix/",
			expectedOperation: "list",
			expectedBucket:    "container",
			expectedKey:       "prefix/",
		},
		{
			name:              "GCP put operation",
			path:              "/-gcpblob/staging/put/bucket/uploads/new.txt",
			expectedOperation: "put",
			expectedBucket:    "bucket",
			expectedKey:       "uploads/new.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation, bucket, key := ParseStorageOperation(tt.path)

			if operation != tt.expectedOperation {
				t.Errorf("operation: got %q, want %q", operation, tt.expectedOperation)
			}
			if bucket != tt.expectedBucket {
				t.Errorf("bucket: got %q, want %q", bucket, tt.expectedBucket)
			}
			if key != tt.expectedKey {
				t.Errorf("key: got %q, want %q", key, tt.expectedKey)
			}
		})
	}
}

// ParseStorageOperation extracts operation, bucket, and key from storage path
func ParseStorageOperation(path string) (operation, bucket, key string) {
	storageType, route, isStorage := ParseStoragePath(path)
	if !isStorage {
		return "", "", ""
	}

	// Path format: /-<type>/<route>/<operation>/<bucket>/<key>
	prefix := "/-" + storageType + "/" + route + "/"
	if len(path) <= len(prefix) {
		return "", "", ""
	}

	remaining := path[len(prefix):]

	// Find operation (next segment)
	slashIdx := -1
	for i, c := range remaining {
		if c == '/' {
			slashIdx = i
			break
		}
	}

	if slashIdx == -1 {
		return remaining, "", ""
	}

	operation = remaining[:slashIdx]
	remaining = remaining[slashIdx+1:]

	// Find bucket (next segment)
	slashIdx = -1
	for i, c := range remaining {
		if c == '/' {
			slashIdx = i
			break
		}
	}

	if slashIdx == -1 {
		return operation, remaining, ""
	}

	bucket = remaining[:slashIdx]
	key = remaining[slashIdx+1:]

	return operation, bucket, key
}
