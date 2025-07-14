#!/bin/bash

# Script to list knowledge files from the Claude Agent Proxy Service

# Claude Agent Proxy Service URL from wavie-services.md
PROXY_URL="https://wavie-claude-proxy-s-455488113475.us-central1.run.app"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Listing knowledge files from Claude Agent Proxy Service${NC}"
echo "============================================================="

# Use curl to list files
RESPONSE=$(curl -s "$PROXY_URL/api/knowledge/files")

echo -e "\n${BLUE}Files:${NC}"
echo "$RESPONSE" | python -m json.tool

echo -e "\n${GREEN}Done!${NC}"
