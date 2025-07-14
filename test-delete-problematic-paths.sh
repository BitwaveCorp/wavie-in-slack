#!/bin/bash

# Test script for deleting documents with problematic paths from the Claude Agent Proxy Service
# This script tests the deletion functionality for the file we just uploaded

# Claude Agent Proxy Service URL from wavie-services.md
PROXY_URL="https://wavie-claude-proxy-s-455488113475.us-central1.run.app"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Testing Claude Agent Proxy Service Deletion with Problematic Paths${NC}"
echo "============================================================="

# Get the file ID from the previous upload
echo -e "\n${BLUE}Enter the file ID from the previous upload:${NC}"
read -p "File ID: " FILE_ID

# Test deleting the file using curl
echo -e "\n${BLUE}Deleting file with ID: $FILE_ID${NC}"
echo "This may take a moment..."

# Use curl to delete the file
RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d "{\"id\":\"$FILE_ID\"}" \
  "$PROXY_URL/api/knowledge/files/delete")

echo -e "\n${BLUE}Delete Response:${NC}"
echo "$RESPONSE" | python -m json.tool

echo -e "\n${GREEN}Test completed!${NC}"
