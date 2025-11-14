#!/bin/bash
set -e

# LLM Proxy Auth - Test Runner Script
# This script helps you run tests at different levels

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "========================================="
echo "  LLM Proxy Auth - Test Suite"
echo "========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to print colored output
print_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

# Check if docker-compose is available
check_docker_compose() {
    if command -v docker-compose &> /dev/null; then
        return 0
    elif docker compose version &> /dev/null 2>&1; then
        # Docker Compose V2 (docker compose)
        return 0
    else
        return 1
    fi
}

# Run docker-compose command (V1 or V2)
run_docker_compose() {
    if command -v docker-compose &> /dev/null; then
        docker-compose "$@"
    else
        docker compose "$@"
    fi
}

# Parse command line arguments
TEST_TYPE="${1:-all}"

case "$TEST_TYPE" in
    unit)
        print_step "Running Unit Tests"
        echo "Testing: Router logic, credential strategies, platform detection"
        echo ""
        go test -v -short ./internal/...
        print_success "Unit tests completed"
        ;;

    coverage)
        print_step "Running Tests with Coverage"
        go test -short -coverprofile=coverage.out ./...
        go tool cover -html=coverage.out -o coverage.html
        print_success "Coverage report generated: coverage.html"

        # Show coverage summary
        echo ""
        print_step "Coverage Summary"
        go tool cover -func=coverage.out | grep total:
        ;;

    integration)
        print_step "Running Integration Tests"

        # Check if Docker is available
        if ! check_docker_compose; then
            print_error "docker-compose not found. Integration tests require Docker."
            exit 1
        fi

        # Start test services
        print_step "Starting test services (LocalStack, Vault, Azurite)..."
        run_docker_compose -f docker-compose.test.yml up -d

        # Wait for services to be ready
        print_step "Waiting for services to be ready..."
        sleep 10

        # Check service health
        print_step "Checking LocalStack health..."
        if curl -s http://localhost:4566/_localstack/health > /dev/null 2>&1; then
            print_success "LocalStack is ready"
        else
            print_warning "LocalStack may not be ready yet"
        fi

        # Run integration tests
        echo ""
        print_step "Running integration tests..."
        go test -tags=integration -v ./test/integration/... || {
            print_error "Integration tests failed"
            print_step "Cleaning up..."
            run_docker_compose -f docker-compose.test.yml down
            exit 1
        }

        print_success "Integration tests completed"

        # Ask if user wants to keep services running
        echo ""
        read -p "Keep test services running? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_step "Stopping test services..."
            run_docker_compose -f docker-compose.test.yml down
            print_success "Test services stopped"
        else
            print_success "Test services still running. Stop with: docker-compose -f docker-compose.test.yml down"
        fi
        ;;

    benchmark)
        print_step "Running Benchmark Tests"
        go test -bench=. -benchmem ./internal/...
        ;;

    race)
        print_step "Running Tests with Race Detector"
        go test -race ./...
        print_success "Race detection completed"
        ;;

    services)
        print_step "Managing Test Services"
        if ! check_docker_compose; then
            print_error "docker-compose not found"
            exit 1
        fi

        ACTION="${2:-up}"

        case "$ACTION" in
            up|start)
                print_step "Starting test services..."
                run_docker_compose -f docker-compose.test.yml up -d
                sleep 5
                print_success "Test services started"
                echo ""
                print_step "Service Endpoints:"
                echo "  - LocalStack (AWS):  http://localhost:4566"
                echo "  - Vault:             http://localhost:8200"
                echo "  - Azurite (Azure):   http://localhost:10000"
                echo "  - MinIO (S3):        http://localhost:9000"
                echo "  - PostgreSQL:        localhost:5432"
                echo "  - Redis:             localhost:6379"
                ;;
            down|stop)
                print_step "Stopping test services..."
                run_docker_compose -f docker-compose.test.yml down
                print_success "Test services stopped"
                ;;
            logs)
                run_docker_compose -f docker-compose.test.yml logs -f
                ;;
            status)
                run_docker_compose -f docker-compose.test.yml ps
                ;;
            *)
                print_error "Unknown action: $ACTION"
                echo "Usage: $0 services [up|down|logs|status]"
                exit 1
                ;;
        esac
        ;;

    all)
        print_step "Running All Tests"

        # Run unit tests first
        echo ""
        print_step "1. Unit Tests"
        go test -short -v ./internal/... || {
            print_error "Unit tests failed"
            exit 1
        }
        print_success "Unit tests passed"

        # Run with coverage
        echo ""
        print_step "2. Coverage Report"
        go test -short -coverprofile=coverage.out ./... > /dev/null 2>&1
        COVERAGE=$(go tool cover -func=coverage.out | grep total: | awk '{print $3}')
        print_success "Code coverage: $COVERAGE"

        # Run race detector
        echo ""
        print_step "3. Race Detection"
        go test -race -short ./... > /dev/null 2>&1
        print_success "No race conditions detected"

        echo ""
        print_success "All tests passed!"
        ;;

    help|--help|-h)
        echo "Usage: $0 [TEST_TYPE]"
        echo ""
        echo "Test Types:"
        echo "  unit          - Run unit tests only (fast)"
        echo "  coverage      - Run tests and generate coverage report"
        echo "  integration   - Run integration tests with Docker services"
        echo "  benchmark     - Run benchmark tests"
        echo "  race          - Run tests with race detector"
        echo "  services      - Manage test services (up|down|logs|status)"
        echo "  all           - Run all tests (default)"
        echo "  help          - Show this help message"
        echo ""
        echo "Examples:"
        echo "  $0                    # Run all tests"
        echo "  $0 unit               # Run only unit tests"
        echo "  $0 integration        # Run integration tests with Docker"
        echo "  $0 coverage           # Generate coverage report"
        echo "  $0 services up        # Start test services"
        echo "  $0 services down      # Stop test services"
        echo ""
        ;;

    *)
        print_error "Unknown test type: $TEST_TYPE"
        echo "Run '$0 help' for usage information"
        exit 1
        ;;
esac
