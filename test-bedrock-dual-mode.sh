#!/bin/bash

# Test script for Bedrock dual-mode architecture
# Requires valid AWS credentials

set -e

echo "========================================="
echo "Bedrock Dual-Mode Architecture Test"
echo "========================================="
echo ""

# Check AWS credentials
echo "Checking AWS credentials..."
if ! aws sts get-caller-identity &> /dev/null; then
    echo "❌ ERROR: AWS credentials not valid"
    echo ""
    echo "Please configure valid AWS credentials:"
    echo "  export AWS_ACCESS_KEY_ID=your_access_key"
    echo "  export AWS_SECRET_ACCESS_KEY=your_secret_key"
    echo "  export AWS_REGION=us-east-1"
    echo ""
    exit 1
fi

IDENTITY=$(aws sts get-caller-identity)
echo "✅ AWS credentials valid"
echo "$IDENTITY" | jq '.'
echo ""

# Check if server is running
if ! curl -s http://localhost:8090/health > /dev/null 2>&1; then
    echo "❌ ERROR: Server not running on port 8090"
    echo "Please start the server first: PORT=8090 ./bin/server"
    exit 1
fi

echo "✅ Server is running"
echo ""

echo "========================================="
echo "Test 1: Protocol Mode (Bedrock via OpenAI API)"
echo "========================================="
echo ""
echo "Endpoint: /openai/bedrock_us1_openai/chat/completions"
echo "Model: anthropic.claude-3-sonnet-20240229-v1:0"
echo ""

PROTOCOL_START=$(date +%s)
PROTOCOL_RESPONSE=$(curl -s -X POST http://localhost:8090/openai/bedrock_us1_openai/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "anthropic.claude-3-sonnet-20240229-v1:0",
    "messages": [
      {"role": "user", "content": "Say hello in exactly 5 words"}
    ],
    "max_tokens": 50
  }')
PROTOCOL_END=$(date +%s)
PROTOCOL_DURATION=$((PROTOCOL_END - PROTOCOL_START))

echo "Response format: OpenAI-compatible (transformed)"
echo ""

if echo "$PROTOCOL_RESPONSE" | jq -e '.choices[0].message.content' > /dev/null 2>&1; then
    echo "✅ Protocol mode test PASSED"
    echo ""
    echo "Response:"
    echo "$PROTOCOL_RESPONSE" | jq '.'
    echo ""
    echo "Content: $(echo "$PROTOCOL_RESPONSE" | jq -r '.choices[0].message.content')"
    echo "Duration: ${PROTOCOL_DURATION}s"
else
    echo "❌ Protocol mode test FAILED"
    echo ""
    echo "Response:"
    echo "$PROTOCOL_RESPONSE" | jq '.'
    exit 1
fi

echo ""
echo "========================================="
echo "Test 2: Transparent Mode (Native Bedrock API)"
echo "========================================="
echo ""
echo "Endpoint: /transparent/bedrock/model/.../converse"
echo "Model: anthropic.claude-3-sonnet-20240229-v1:0"
echo ""

TRANSPARENT_START=$(date +%s)
TRANSPARENT_RESPONSE=$(curl -s -X POST "http://localhost:8090/transparent/bedrock/model/anthropic.claude-3-sonnet-20240229-v1:0/converse" \
  -H 'Content-Type: application/json' \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": [{"text": "Say goodbye in exactly 5 words"}]
      }
    ],
    "inferenceConfig": {
      "maxTokens": 50,
      "temperature": 0.7
    }
  }')
TRANSPARENT_END=$(date +%s)
TRANSPARENT_DURATION=$((TRANSPARENT_END - TRANSPARENT_START))

echo "Response format: Native Bedrock Converse API (not transformed)"
echo ""

if echo "$TRANSPARENT_RESPONSE" | jq -e '.output.message.content[0].text' > /dev/null 2>&1; then
    echo "✅ Transparent mode test PASSED"
    echo ""
    echo "Response:"
    echo "$TRANSPARENT_RESPONSE" | jq '.'
    echo ""
    echo "Content: $(echo "$TRANSPARENT_RESPONSE" | jq -r '.output.message.content[0].text')"
    echo "Duration: ${TRANSPARENT_DURATION}s"
else
    echo "❌ Transparent mode test FAILED"
    echo ""
    echo "Response:"
    echo "$TRANSPARENT_RESPONSE" | jq '.'
    exit 1
fi

echo ""
echo "========================================="
echo "Summary"
echo "========================================="
echo ""
echo "✅ Protocol Mode (OpenAI-compatible):"
echo "   - Request: OpenAI chat completions format"
echo "   - Response: OpenAI format (transformed from Bedrock)"
echo "   - Duration: ${PROTOCOL_DURATION}s"
echo ""
echo "✅ Transparent Mode (Native Bedrock):"
echo "   - Request: Bedrock Converse API format"
echo "   - Response: Bedrock Converse format (not transformed)"
echo "   - Duration: ${TRANSPARENT_DURATION}s"
echo ""
echo "Key Differences:"
echo "  • Protocol mode provides OpenAI-compatible API"
echo "  • Transparent mode preserves native Bedrock API"
echo "  • Both authenticate with AWS SigV4 automatically"
echo ""
echo "========================================="
echo "All tests completed successfully! ✅"
echo "========================================="
