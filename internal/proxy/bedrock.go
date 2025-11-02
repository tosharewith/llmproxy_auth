package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/tosharewith/llmproxy_auth/internal/auth"
	"github.com/tosharewith/llmproxy_auth/internal/health"
	"github.com/tosharewith/llmproxy_auth/pkg/metrics"
	"github.com/gin-gonic/gin"
)

// BedrockProxy handles proxying requests to AWS Bedrock
type BedrockProxy struct {
	signer        *auth.AWSSigner
	proxy         *httputil.ReverseProxy
	target        *url.URL
	healthChecker *health.Checker
}

// NewBedrockProxy creates a new Bedrock proxy with embedded IAM authentication
func NewBedrockProxy(region string, healthChecker *health.Checker) (*BedrockProxy, error) {
	signer, err := auth.NewAWSSigner(region, "bedrock")
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS signer: %w", err)
	}

	target, err := url.Parse(fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", region))
	if err != nil {
		return nil, fmt.Errorf("failed to parse target URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	bp := &BedrockProxy{
		signer:        signer,
		proxy:         proxy,
		target:        target,
		healthChecker: healthChecker,
	}

	// Configure custom director for request signing
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		bp.directRequest(originalDirector, req)
	}

	// Configure error handler
	proxy.ErrorHandler = bp.errorHandler

	// Configure response modifier
	proxy.ModifyResponse = bp.modifyResponse

	return bp, nil
}

// Handler returns a Gin handler for Bedrock requests
func (bp *BedrockProxy) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Validate request
		if err := bp.validateRequest(c); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request",
				"message": err.Error(),
			})
			return
		}

		// Prepare request path
		c.Request.URL.Path = bp.preparePath(c.Request.URL.Path)

		// Set target host
		c.Request.Host = bp.target.Host
		c.Request.URL.Host = bp.target.Host
		c.Request.URL.Scheme = bp.target.Scheme

		// Create response recorder for metrics
		recorder := &responseRecorder{
			ResponseWriter: c.Writer,
			statusCode:     200,
		}
		c.Writer = recorder

		// Proxy the request
		bp.proxy.ServeHTTP(c.Writer, c.Request)

		// Record metrics
		duration := time.Since(start)
		status := fmt.Sprintf("%d", recorder.statusCode)
		method := c.Request.Method

		metrics.RequestDuration.WithLabelValues(method, status).Observe(duration.Seconds())
		metrics.RequestsTotal.WithLabelValues(method, status).Inc()

		// Update health checker
		if recorder.statusCode >= 500 {
			bp.healthChecker.RecordError()
		} else {
			bp.healthChecker.RecordSuccess()
		}
	}
}

// directRequest configures the request for AWS Bedrock
func (bp *BedrockProxy) directRequest(originalDirector func(*http.Request), req *http.Request) {
	// Call original director
	originalDirector(req)

	// Read and preserve request body for signing
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Add required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "bedrock-proxy/1.0")

	// Sign the request with AWS Signature V4
	if err := bp.signer.SignRequest(req, bodyBytes); err != nil {
		log.Printf("Failed to sign request: %v", err)
	}
}

// validateRequest performs basic request validation
func (bp *BedrockProxy) validateRequest(c *gin.Context) error {
	// Check Content-Type for POST/PUT requests
	if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut {
		contentType := c.GetHeader("Content-Type")
		if contentType != "" && !strings.Contains(contentType, "application/json") {
			return fmt.Errorf("unsupported content type: %s", contentType)
		}
	}

	// Validate request size (max 1MB)
	if c.Request.ContentLength > 1024*1024 {
		return fmt.Errorf("request body too large: %d bytes", c.Request.ContentLength)
	}

	return nil
}

// preparePath prepares the request path for Bedrock
func (bp *BedrockProxy) preparePath(path string) string {
	// Remove proxy prefix
	path = strings.TrimPrefix(path, "/v1/bedrock")
	path = strings.TrimPrefix(path, "/bedrock")

	// Ensure path starts with "/"
	if path == "" || !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}

// errorHandler handles proxy errors
func (bp *BedrockProxy) errorHandler(rw http.ResponseWriter, req *http.Request, err error) {
	log.Printf("Proxy error: %v", err)

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusBadGateway)

	response := gin.H{
		"error":   "Bedrock service unavailable",
		"message": "The Bedrock service is currently unavailable. Please try again later.",
	}

	// Don't expose internal errors in production
	if gin.Mode() == gin.DebugMode {
		response["debug"] = err.Error()
	}

	// Write JSON response (simple approach)
	rw.Write([]byte(`{"error":"Bedrock service unavailable","message":"The Bedrock service is currently unavailable. Please try again later."}`))

	// Record error in health checker
	bp.healthChecker.RecordError()
}

// modifyResponse modifies the response from Bedrock
func (bp *BedrockProxy) modifyResponse(resp *http.Response) error {
	// Add security headers
	resp.Header.Set("X-Content-Type-Options", "nosniff")
	resp.Header.Set("X-Frame-Options", "DENY")
	resp.Header.Set("X-XSS-Protection", "1; mode=block")

	// Add CORS headers if needed
	resp.Header.Set("Access-Control-Allow-Origin", "*")
	resp.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	resp.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	return nil
}

// responseRecorder captures response status for metrics
type responseRecorder struct {
	gin.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}
