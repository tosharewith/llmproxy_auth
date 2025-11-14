# Testing Quick Start

**Run tests in 3 simple steps:**

## 1. Quick Unit Tests (No Setup Required)

Test the router and credential logic right now:

```bash
# Test everything
go test ./...

# Or use the test script
./scripts/run-tests.sh unit
```

**What this tests:**
- ✅ Storage path routing (`/-s3/`, `/-azblob/`, etc.)
- ✅ Platform detection (EKS, AKS, GKE, OKE)
- ✅ Credential strategy selection
- ✅ Storage operation parsing (get, put, presign, list)

**Output you'll see:**
```
=== RUN   TestParseStoragePath
=== RUN   TestParseStoragePath/S3_storage_path
=== RUN   TestParseStoragePath/Azure_Blob_storage_path
--- PASS: TestParseStoragePath (0.00s)

=== RUN   TestPlatformDetection
=== RUN   TestPlatformDetection/Detect_AWS_EKS_with_IRSA
=== RUN   TestPlatformDetection/Detect_Azure_AKS_with_Managed_Identity
--- PASS: TestPlatformDetection (0.00s)

PASS
```

## 2. Integration Tests with Docker (Optional)

Test S3, Azure Blob, and vault operations with mock services:

```bash
# Start test services (LocalStack, Azurite, Vault, MinIO)
./scripts/run-tests.sh services up

# Run integration tests
./scripts/run-tests.sh integration

# Or manually:
docker-compose -f docker-compose.test.yml up -d
go test -tags=integration ./test/integration/
```

**What this tests:**
- ✅ S3 operations (create bucket, put, get, list, delete)
- ✅ Pre-signed URL generation
- ✅ Azure Blob Storage operations
- ✅ HashiCorp Vault credential retrieval
- ✅ AWS Secrets Manager (via LocalStack)

**Services started:**
- LocalStack (AWS services): `http://localhost:4566`
- Vault: `http://localhost:8200`
- Azurite (Azure Blob): `http://localhost:10000`
- MinIO (S3-compatible): `http://localhost:9000`

## 3. Coverage Report

See which code is tested:

```bash
# Generate coverage report
./scripts/run-tests.sh coverage

# Opens coverage.html in browser
open coverage.html
```

---

## Test Script Usage

The `./scripts/run-tests.sh` script makes testing easy:

```bash
# Run all tests
./scripts/run-tests.sh

# Run specific test types
./scripts/run-tests.sh unit         # Fast unit tests only
./scripts/run-tests.sh coverage     # Generate coverage report
./scripts/run-tests.sh integration  # Integration tests with Docker
./scripts/run-tests.sh benchmark    # Performance benchmarks
./scripts/run-tests.sh race         # Race condition detection

# Manage test services
./scripts/run-tests.sh services up     # Start Docker services
./scripts/run-tests.sh services down   # Stop Docker services
./scripts/run-tests.sh services logs   # View service logs
./scripts/run-tests.sh services status # Check service status
```

---

## What Gets Tested

### ✅ Currently Working Tests

| Component | What's Tested | Run With |
|-----------|--------------|----------|
| **Path Router** | Storage path detection (`/-s3/`, `/-azblob/`) | `go test ./internal/router/` |
| **Path Parser** | Operation extraction (presign, get, put) | `go test ./internal/router/` |
| **Platform Detection** | EKS, AKS, GKE, OKE, generic K8s | `go test ./internal/credentials/` |
| **Strategy Selection** | Workload identity → Vault → K8s Secret | `go test ./internal/credentials/` |
| **S3 Operations** | Full CRUD + presigned URLs | `go test -tags=integration ./test/integration/` |

### 🚧 Ready to Implement

These test files exist in `docs/TESTING.md` and can be implemented:

- Azure Blob operations
- GCP Cloud Storage operations
- Multi-vault backend (HashiCorp, AWS SM, Azure KV, GCP SM)
- End-to-end AI provider requests
- RAG integration (document upload → presign → AI request)
- Performance/load testing with k6

---

## Quick Examples

### Test Storage Path Routing

```bash
$ go test -v -run TestParseStoragePath ./internal/router/

=== RUN   TestParseStoragePath/S3_storage_path
    Path: /-s3/prod/presign/my-bucket/document.pdf
    Type: s3, Route: prod, IsStorage: true
    ✅ PASS

=== RUN   TestParseStoragePath/Azure_Blob_storage_path
    Path: /-azblob/dev/get/container/file.txt
    Type: azblob, Route: dev, IsStorage: true
    ✅ PASS
```

### Test Platform Detection

```bash
$ go test -v -run TestPlatformDetection ./internal/credentials/

=== RUN   TestPlatformDetection/Detect_AWS_EKS_with_IRSA
    Platform: eks
    Features: aws_workload_identity=true
    ✅ PASS

=== RUN   TestPlatformDetection/Detect_Azure_AKS_with_Managed_Identity
    Platform: aks
    Features: azure_workload_identity=true
    ✅ PASS
```

### Test S3 Integration

```bash
$ docker-compose -f docker-compose.test.yml up -d
$ go test -v -tags=integration -run TestLocalStackS3 ./test/integration/

=== RUN   TestLocalStackS3Integration
=== RUN   TestLocalStackS3Integration/CreateBucket
    ✅ Created bucket: test-bucket-20250114103045
=== RUN   TestLocalStackS3Integration/PutObject
    ✅ Uploaded object: test-files/document.txt
=== RUN   TestLocalStackS3Integration/GetObject
    ✅ Retrieved object with correct content
=== RUN   TestLocalStackS3Integration/PresignedURL
    ✅ Generated presigned URL
```

---

## Testing Different Scenarios

### Scenario 1: Test on Different Platforms

Simulate running on different Kubernetes platforms:

```bash
# Simulate AWS EKS
export AWS_ROLE_ARN=arn:aws:iam::123:role/test
export AWS_WEB_IDENTITY_TOKEN_FILE=/tmp/token
go test -run TestPlatformDetection ./internal/credentials/

# Simulate Azure AKS
unset AWS_ROLE_ARN AWS_WEB_IDENTITY_TOKEN_FILE
export AZURE_CLIENT_ID=12345
export AZURE_FEDERATED_TOKEN_FILE=/tmp/token
go test -run TestPlatformDetection ./internal/credentials/

# Simulate GCP GKE
unset AZURE_CLIENT_ID AZURE_FEDERATED_TOKEN_FILE
export GOOGLE_APPLICATION_CREDENTIALS=/tmp/sa.json
go test -run TestPlatformDetection ./internal/credentials/
```

### Scenario 2: Test Credential Fallback

Test that strategies fall back correctly:

```bash
go test -v -run TestCredentialStrategySelection ./internal/credentials/

# Output shows:
# EKS → workload_identity (best)
# AKS → workload_identity (best)
# Generic K8s → vault (fallback)
# Generic K8s (no vault) → k8s_secret (last resort)
```

### Scenario 3: Test Storage Operations

Test all storage operations:

```bash
# Start LocalStack
docker-compose -f docker-compose.test.yml up -d

# Run S3 tests
go test -v -tags=integration -run TestLocalStackS3 ./test/integration/

# Tests run:
# 1. CreateBucket ✅
# 2. PutObject ✅
# 3. GetObject ✅
# 4. ListObjects ✅
# 5. PresignedURL ✅
# 6. DeleteObject ✅
# 7. DeleteBucket ✅
```

---

## Continuous Integration

Add to `.github/workflows/test.yml`:

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Unit Tests
        run: ./scripts/run-tests.sh unit

      - name: Coverage
        run: ./scripts/run-tests.sh coverage

      - name: Integration Tests
        run: ./scripts/run-tests.sh integration
```

---

## Troubleshooting

### Tests Failing?

```bash
# Check Go version
go version  # Should be 1.21+

# Clean and retry
go clean -testcache
go test ./...
```

### Docker Services Not Starting?

```bash
# Check Docker is running
docker ps

# Check service health
curl http://localhost:4566/_localstack/health
curl http://localhost:8200/v1/sys/health

# View logs
docker-compose -f docker-compose.test.yml logs
```

### Integration Tests Timing Out?

```bash
# Services may need more time to start
docker-compose -f docker-compose.test.yml up -d
sleep 15  # Wait longer
go test -tags=integration ./test/integration/
```

---

## Next Steps

1. **Start Simple**: Run `go test ./...` to see current tests pass
2. **Try Integration**: Start Docker services and run integration tests
3. **Add More Tests**: Use templates in `docs/TESTING.md` to add new tests
4. **Implement Features**: Tests are ready, now build the actual handlers!

For comprehensive testing documentation, see **[docs/TESTING.md](docs/TESTING.md)**
