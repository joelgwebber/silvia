#!/bin/bash

# Test script for Silvia extension API

echo "Testing Silvia Extension API..."
echo "================================"
echo ""

# Test status endpoint
echo "1. Testing /api/status endpoint..."
STATUS_RESPONSE=$(curl -s http://localhost:8765/api/status)

if [ $? -eq 0 ]; then
    echo "✓ Server is running"
    echo "Response: $STATUS_RESPONSE"
else
    echo "✗ Server is not running. Please start Silvia first:"
    echo "  ./bin/silvia"
    exit 1
fi

echo ""
echo "2. Testing /api/ingest endpoint with sample data..."

# Sample ingestion data
INGEST_DATA='{
  "url": "https://example.com/test",
  "title": "Test Article from Extension",
  "html": "<html><body><h1>Test Content</h1><p>This is a test article with a <a href=\"https://example.com/link\">link</a>.</p></body></html>",
  "text": "Test Content\nThis is a test article with a link.",
  "links": [
    {
      "url": "https://example.com/link",
      "text": "link",
      "context": "This is a test article with a link."
    }
  ],
  "metadata": {
    "author": "Test Author",
    "date": "2024-01-01",
    "platform": "test"
  }
}'

echo "Sending test ingestion request..."
INGEST_RESPONSE=$(curl -s -X POST http://localhost:8765/api/ingest \
  -H "Content-Type: application/json" \
  -d "$INGEST_DATA")

if [ $? -eq 0 ]; then
    echo "✓ Ingestion request sent"
    echo "Response: $INGEST_RESPONSE"
else
    echo "✗ Ingestion request failed"
fi

echo ""
echo "================================"
echo "API test complete!"