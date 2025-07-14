#!/bin/bash

# Test script for deletion with verbose output
# This script tests the deletion with verbose output to see the detailed logs

# Claude Agent Proxy Service URL from wavie-services.md
PROXY_URL="https://wavie-claude-proxy-s-455488113475.us-central1.run.app"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Testing Deletion with Verbose Output${NC}"
echo "============================================================="

# Get the file ID from the user
echo -e "\n${BLUE}Enter the file ID to delete:${NC}"
read -p "File ID: " FILE_ID

# Test deleting the file using curl with -v flag to see the HTTP status code
echo -e "\n${BLUE}Deleting file with ID: $FILE_ID${NC}"
echo "This may take a moment..."

# Use curl with verbose output and increased timeout
curl -v -X POST \
  --max-time 180 \
  -H "Content-Type: application/json" \
  -d "{\"id\":\"$FILE_ID\"}" \
  "$PROXY_URL/api/knowledge/files/delete" 2>&1

echo -e "\n${GREEN}Test completed!${NC}"

# Now let's check the logs for this deletion
echo -e "\n${BLUE}Checking logs for deletion (if available)${NC}"
echo "============================================================="
echo "Note: You may need to check the Cloud Run logs in the Google Cloud Console for detailed logs."
