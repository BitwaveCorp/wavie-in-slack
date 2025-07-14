#!/bin/bash

# Test script for asynchronous deletion
# This script tests the asynchronous deletion by checking if the 202 Accepted status is returned

# Claude Agent Proxy Service URL from wavie-services.md
PROXY_URL="https://wavie-claude-proxy-s-455488113475.us-central1.run.app"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Testing Asynchronous Deletion${NC}"
echo "============================================================="

# Get the file ID from the user
echo -e "\n${BLUE}Enter the file ID to delete:${NC}"
read -p "File ID: " FILE_ID

# Test deleting the file using curl with -v flag to see the HTTP status code
echo -e "\n${BLUE}Deleting file with ID: $FILE_ID${NC}"
echo "This may take a moment..."

# Use curl with verbose output to see the HTTP status code
curl -v -X POST \
  --max-time 180 \
  -H "Content-Type: application/json" \
  -d "{\"id\":\"$FILE_ID\"}" \
  "$PROXY_URL/api/knowledge/files/delete" 2>&1 | grep -E "< HTTP|{.*}"

echo -e "\n${GREEN}Test completed!${NC}"
