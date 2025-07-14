#!/bin/bash

# Test script for sanitized document IDs in RAG service
# This script tests the upload and deletion of documents with problematic paths

# RAG service URL
RAG_URL="https://wavie-rag-svc-455488113475.us-central1.run.app"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Testing RAG Service with Sanitized Document IDs${NC}"
echo "=================================================="

# Test 1: Upload a document with problematic path characters
echo -e "\n${BLUE}Test 1: Uploading test document with problematic path${NC}"
curl -X POST -H "Content-Type: application/json" -d '{
  "document_id": "test-sanitized/path.with/special/chars",
  "file_path": "test/test-sanitized-path.md"
}' $RAG_URL/api/documents

# Test 2: Upload documents with common prefix but problematic paths
echo -e "\n\n${BLUE}Test 2: Uploading documents with common prefix and problematic paths${NC}"
curl -X POST -H "Content-Type: application/json" -d '{
  "document_id": "test-prefix/path.1",
  "file_path": "test/test-prefix-1.md"
}' $RAG_URL/api/documents

echo -e "\n"
curl -X POST -H "Content-Type: application/json" -d '{
  "document_id": "test-prefix/path.2",
  "file_path": "test/test-prefix-2.md"
}' $RAG_URL/api/documents

echo -e "\n"
curl -X POST -H "Content-Type: application/json" -d '{
  "document_id": "test-prefix/path[3]",
  "file_path": "test/test-prefix-3.md"
}' $RAG_URL/api/documents

# Test 3: Delete a single document with problematic path
echo -e "\n\n${BLUE}Test 3: Deleting single document with problematic path${NC}"
curl -X DELETE "$RAG_URL/api/documents/test-sanitized-path_with-special-chars"

# Test 4: Delete multiple documents with prefix and problematic paths
echo -e "\n\n${BLUE}Test 4: Deleting multiple documents with prefix and problematic paths${NC}"
curl -X DELETE "$RAG_URL/api/documents/prefix/test-prefix"

# Test 5: Health check to verify service is still running
echo -e "\n\n${BLUE}Test 5: Verifying service health${NC}"
curl -s $RAG_URL/health

echo -e "\n\n${GREEN}Tests completed!${NC}"
