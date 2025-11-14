// +build integration

package integration

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// TestLocalStackS3Integration tests S3 operations against LocalStack
// Run with: go test -tags=integration ./test/integration/
func TestLocalStackS3Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create S3 client pointing to LocalStack
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test",
			"test",
			"",
		)),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               "http://localhost:4566",
					SigningRegion:     "us-east-1",
					HostnameImmutable: true,
				}, nil
			},
		)),
	)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	client := s3.NewFromConfig(cfg)

	bucketName := "test-bucket-" + time.Now().Format("20060102150405")
	testKey := "test-files/document.txt"
	testContent := "Hello from integration test!"

	// Test 1: Create bucket
	t.Run("CreateBucket", func(t *testing.T) {
		_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			t.Fatalf("Failed to create bucket: %v", err)
		}
		t.Logf("✅ Created bucket: %s", bucketName)
	})

	// Test 2: Put object
	t.Run("PutObject", func(t *testing.T) {
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(testKey),
			Body:   strings.NewReader(testContent),
		})
		if err != nil {
			t.Fatalf("Failed to put object: %v", err)
		}
		t.Logf("✅ Uploaded object: %s", testKey)
	})

	// Test 3: Get object
	t.Run("GetObject", func(t *testing.T) {
		result, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(testKey),
		})
		if err != nil {
			t.Fatalf("Failed to get object: %v", err)
		}
		defer result.Body.Close()

		data, err := io.ReadAll(result.Body)
		if err != nil {
			t.Fatalf("Failed to read object: %v", err)
		}

		if string(data) != testContent {
			t.Errorf("Content mismatch: got %q, want %q", string(data), testContent)
		}
		t.Logf("✅ Retrieved object with correct content")
	})

	// Test 4: List objects
	t.Run("ListObjects", func(t *testing.T) {
		result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(bucketName),
			Prefix: aws.String("test-files/"),
		})
		if err != nil {
			t.Fatalf("Failed to list objects: %v", err)
		}

		if len(result.Contents) != 1 {
			t.Errorf("Expected 1 object, got %d", len(result.Contents))
		}
		if len(result.Contents) > 0 && *result.Contents[0].Key != testKey {
			t.Errorf("Key mismatch: got %q, want %q", *result.Contents[0].Key, testKey)
		}
		t.Logf("✅ Listed %d objects", len(result.Contents))
	})

	// Test 5: Generate presigned URL
	t.Run("PresignedURL", func(t *testing.T) {
		presignClient := s3.NewPresignClient(client)

		presignResult, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(testKey),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = 1 * time.Hour
		})
		if err != nil {
			t.Fatalf("Failed to generate presigned URL: %v", err)
		}

		if presignResult.URL == "" {
			t.Error("Presigned URL is empty")
		}
		if !strings.Contains(presignResult.URL, bucketName) {
			t.Errorf("Presigned URL doesn't contain bucket name: %s", presignResult.URL)
		}
		t.Logf("✅ Generated presigned URL: %s", presignResult.URL[:80]+"...")
	})

	// Test 6: Delete object
	t.Run("DeleteObject", func(t *testing.T) {
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(testKey),
		})
		if err != nil {
			t.Fatalf("Failed to delete object: %v", err)
		}
		t.Logf("✅ Deleted object: %s", testKey)
	})

	// Cleanup: Delete bucket
	t.Run("DeleteBucket", func(t *testing.T) {
		_, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			t.Fatalf("Failed to delete bucket: %v", err)
		}
		t.Logf("✅ Deleted bucket: %s", bucketName)
	})
}
