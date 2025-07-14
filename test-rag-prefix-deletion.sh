#!/bin/bash

# Test script for RAG service prefix-based deletion
# This script tests both standard document deletion and prefix-based deletion

# RAG service URL
RAG_URL="https://wavie-rag-svc-455488113475.us-central1.run.app"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Testing RAG Service Prefix-Based Deletion${NC}"
echo "=================================================="

# Test 1: Upload a test document
echo -e "\n${BLUE}Test 1: Uploading test document 'test-prefix-doc'${NC}"
curl -X POST -H "Content-Type: application/json" -d '{
  "document_id": "test-prefix-doc",
  "file_path": "test/test-prefix-doc.md"
}' $RAG_URL/api/documents

# Test 2: Upload documents with a common prefix
echo -e "\n\n${BLUE}Test 2: Uploading documents with common prefix 'test-prefix-'${NC}"
curl -X POST -H "Content-Type: application/json" -d '{
  "document_id": "test-prefix-1",
  "file_path": "test/test-prefix-1.md"
}' $RAG_URL/api/documents

echo -e "\n"
curl -X POST -H "Content-Type: application/json" -d '{
  "document_id": "test-prefix-2",
  "file_path": "test/test-prefix-2.md"
}' $RAG_URL/api/documents

echo -e "\n"
curl -X POST -H "Content-Type: application/json" -d '{
  "document_id": "test-prefix-3",
  "file_path": "test/test-prefix-3.md"
}' $RAG_URL/api/documents

# Test 3: Delete a single document using standard deletion
echo -e "\n\n${BLUE}Test 3: Deleting single document 'test-prefix-doc' using standard deletion${NC}"
curl -X DELETE $RAG_URL/api/documents/test-prefix-doc

# Test 4: Delete multiple documents using prefix-based deletion
echo -e "\n\n${BLUE}Test 4: Deleting multiple documents with prefix 'test-prefix-' using prefix-based deletion${NC}"
curl -X DELETE $RAG_URL/api/documents/prefix/test-prefix-

# Test 5: Health check to verify service is still running
echo -e "\n\n${BLUE}Test 5: Verifying service health${NC}"
curl -s $RAG_URL/health

echo -e "\n\n${GREEN}Tests completed!${NC}"
