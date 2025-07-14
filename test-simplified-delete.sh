#!/bin/bash

# Test script for simplified deletion approach
# This script tests the deletion of files using the standard delete endpoint

# Claude Agent Proxy Service URL from wavie-services.md
PROXY_URL="https://wavie-claude-proxy-s-455488113475.us-central1.run.app"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Testing Simplified Deletion Approach${NC}"
echo "============================================================="

# Get the file ID from the user
echo -e "\n${BLUE}Enter the file ID to delete:${NC}"
read -p "File ID: " FILE_ID

# Test deleting the file using curl
echo -e "\n${BLUE}Deleting file with ID: $FILE_ID${NC}"
echo "This may take a moment..."

# Use curl to delete the file with increased timeout (180 seconds)
RESPONSE=$(curl -s -X POST \
  --max-time 180 \
  -H "Content-Type: application/json" \
  -d "{\"id\":\"$FILE_ID\"}" \
  "$PROXY_URL/api/knowledge/files/delete")

echo -e "\n${BLUE}Delete Response:${NC}"
echo "$RESPONSE" | python -m json.tool

echo -e "\n${GREEN}Test completed!${NC}"
