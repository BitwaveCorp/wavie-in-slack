#!/bin/bash

# Test file deletion with the Claude Agent Proxy Service
# This script sends a delete request to the Claude Agent Proxy Service

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Service URLs
CLAUDE_PROXY_URL="https://wavie-claude-proxy-s-455488113475.us-central1.run.app"
RAG_SERVICE_URL="https://wavie-rag-svc-455488113475.us-central1.run.app"

# File ID to delete (replace with your actual file ID)
FILE_ID="$1"

if [ -z "$FILE_ID" ]; then
    echo -e "${RED}Error: File ID is required${NC}"
    echo -e "Usage: $0 <file_id>"
    exit 1
fi

echo -e "${BLUE}Testing deletion of file with ID: ${FILE_ID}${NC}"
echo -e "${BLUE}Claude Proxy URL: ${CLAUDE_PROXY_URL}${NC}"
echo -e "${BLUE}RAG Service URL: ${RAG_SERVICE_URL}${NC}"

# Send delete request to Claude Agent Proxy Service
echo -e "\n${BLUE}Sending delete request...${NC}"
RESPONSE=$(curl -s -X POST \
    -H "Content-Type: application/json" \
    -d "{\"id\": \"${FILE_ID}\"}" \
    "${CLAUDE_PROXY_URL}/api/knowledge/files/delete")

# Print the response
echo -e "\n${GREEN}Response:${NC}"
echo $RESPONSE | jq -r '.'

# Check if the response indicates success
SUCCESS=$(echo $RESPONSE | jq -r '.success')
if [ "$SUCCESS" == "true" ]; then
    echo -e "\n${GREEN}Delete request accepted. File deletion is running in the background.${NC}"
    echo -e "${BLUE}Details: $(echo $RESPONSE | jq -r '.details')${NC}"
    
    # Check if RAG service deletion was attempted
    RAG_ENABLED=$(echo $RESPONSE | jq -r '.rag.enabled')
    if [ "$RAG_ENABLED" == "true" ]; then
        echo -e "\n${GREEN}RAG service deletion was initiated.${NC}"
        echo -e "${BLUE}Attempted: $(echo $RESPONSE | jq -r '.rag.attempted')${NC}"
        echo -e "${BLUE}Successful: $(echo $RESPONSE | jq -r '.rag.successful')${NC}"
        echo -e "${BLUE}Failed: $(echo $RESPONSE | jq -r '.rag.failed')${NC}"
        
        if [ "$(echo $RESPONSE | jq -r '.rag.error_message')" != "null" ]; then
            echo -e "${RED}Error message: $(echo $RESPONSE | jq -r '.rag.error_message')${NC}"
        fi
    else
        echo -e "\n${RED}RAG service deletion was not enabled.${NC}"
    fi
else
    echo -e "\n${RED}Delete request failed.${NC}"
    echo -e "${RED}Error: $(echo $RESPONSE | jq -r '.error')${NC}"
    echo -e "${RED}Details: $(echo $RESPONSE | jq -r '.details')${NC}"
fi
