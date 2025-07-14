#!/bin/bash

# Test script for simplified deletion approach with local service
# This script tests the deletion of files using the standard delete endpoint

# Local Claude Agent Proxy Service URL
PROXY_URL="http://localhost:8080"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Testing Simplified Deletion Approach (Local)${NC}"
echo "============================================================="

# Get the file ID from the user
echo -e "\n${BLUE}Enter the file ID to delete:${NC}"
read -p "File ID: " FILE_ID

# Test deleting the file using curl
echo -e "\n${BLUE}Deleting file with ID: $FILE_ID${NC}"
echo "This may take a moment..."

# Use curl to delete the file
RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d "{\"id\":\"$FILE_ID\"}" \
  "$PROXY_URL/api/knowledge/files/delete")

# Display the response
echo -e "\n${BLUE}Delete Response:${NC}"
echo "$RESPONSE" | jq -C . || echo "$RESPONSE"

echo -e "\n${GREEN}Test completed!${NC}"
