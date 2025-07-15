#!/bin/bash

# Test script for the "one active file per bot" approach
# This script will:
# 1. Upload a ZIP file for a specific agent
# 2. Verify it's set as the active file
# 3. Upload a second ZIP file for the same agent
# 4. Verify the second file replaces the first as the active file
# 5. Test the soft deletion mechanism

set -e

# Configuration
API_URL="http://localhost:8080"
AGENT_ID="test-agent-1"  # Replace with an actual agent ID from your system

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Testing one active file per bot implementation...${NC}"

# Create a temporary directory for test files
TEMP_DIR=$(mktemp -d)
echo "Created temporary directory: $TEMP_DIR"
trap 'rm -rf "$TEMP_DIR"' EXIT

# Create first test ZIP file
echo -e "${BLUE}Creating first test ZIP file...${NC}"
mkdir -p "$TEMP_DIR/test-file-1"
echo "This is test file 1 content" > "$TEMP_DIR/test-file-1/test1.md"
(cd "$TEMP_DIR" && zip -r test-file-1.zip test-file-1)
echo -e "${GREEN}Created first test ZIP file${NC}"

# Create second test ZIP file
echo -e "${BLUE}Creating second test ZIP file...${NC}"
mkdir -p "$TEMP_DIR/test-file-2"
echo "This is test file 2 content" > "$TEMP_DIR/test-file-2/test2.md"
(cd "$TEMP_DIR" && zip -r test-file-2.zip test-file-2)
echo -e "${GREEN}Created second test ZIP file${NC}"

# Step 1: Upload first ZIP file
echo -e "${BLUE}Uploading first test file...${NC}"
UPLOAD_RESPONSE=$(curl -s -X POST \
  -F "name=Test File 1" \
  -F "description=Test file for active file implementation" \
  -F "agent_ids=$AGENT_ID" \
  -F "file=@$TEMP_DIR/test-file-1.zip" \
  "$API_URL/api/knowledge/files")

# Extract file ID from response
FILE_ID_1=$(echo $UPLOAD_RESPONSE | grep -o '"id":"[^"]*' | cut -d'"' -f4)

if [ -z "$FILE_ID_1" ]; then
  echo -e "${RED}Failed to upload first file or extract file ID${NC}"
  echo "Response: $UPLOAD_RESPONSE"
  exit 1
fi

echo -e "${GREEN}Successfully uploaded first file with ID: $FILE_ID_1${NC}"

# Step 2: Verify it's set as the active file
echo -e "${BLUE}Verifying first file is set as active...${NC}"
AGENTS_RESPONSE=$(curl -s "$API_URL/api/knowledge/agents")

# Check if the agent has the correct active file ID
if echo "$AGENTS_RESPONSE" | grep -q "\"id\":\"$AGENT_ID\"" && echo "$AGENTS_RESPONSE" | grep -q "\"active_knowledge_file_id\":\"$FILE_ID_1\""; then
  echo -e "${GREEN}First file successfully set as active for agent $AGENT_ID${NC}"
else
  echo -e "${RED}First file not set as active for agent $AGENT_ID${NC}"
  echo "Agents response: $AGENTS_RESPONSE"
  exit 1
fi

# Step 3: Upload second ZIP file
echo -e "${BLUE}Uploading second test file...${NC}"
UPLOAD_RESPONSE=$(curl -s -X POST \
  -F "name=Test File 2" \
  -F "description=Second test file for active file implementation" \
  -F "agent_ids=$AGENT_ID" \
  -F "file=@$TEMP_DIR/test-file-2.zip" \
  "$API_URL/api/knowledge/files")

# Extract file ID from response
FILE_ID_2=$(echo $UPLOAD_RESPONSE | grep -o '"id":"[^"]*' | cut -d'"' -f4)

if [ -z "$FILE_ID_2" ]; then
  echo -e "${RED}Failed to upload second file or extract file ID${NC}"
  echo "Response: $UPLOAD_RESPONSE"
  exit 1
fi

echo -e "${GREEN}Successfully uploaded second file with ID: $FILE_ID_2${NC}"

# Step 4: Verify the second file replaces the first as the active file
echo -e "${BLUE}Verifying second file is now set as active...${NC}"
AGENTS_RESPONSE=$(curl -s "$API_URL/api/knowledge/agents")

# Check if the agent has the correct active file ID
if echo "$AGENTS_RESPONSE" | grep -q "\"id\":\"$AGENT_ID\"" && echo "$AGENTS_RESPONSE" | grep -q "\"active_knowledge_file_id\":\"$FILE_ID_2\""; then
  echo -e "${GREEN}Second file successfully set as active for agent $AGENT_ID${NC}"
else
  echo -e "${RED}Second file not set as active for agent $AGENT_ID${NC}"
  echo "Agents response: $AGENTS_RESPONSE"
  exit 1
fi

# Step 5: Test the soft deletion mechanism for the second file
echo -e "${BLUE}Testing soft deletion of the second file...${NC}"
DELETE_RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d "{\"ID\":\"$FILE_ID_2\"}" \
  "$API_URL/api/knowledge/files/delete")

echo "Delete response: $DELETE_RESPONSE"

# Verify the file is removed from the registry
echo -e "${BLUE}Verifying file is removed from registry...${NC}"
FILES_RESPONSE=$(curl -s "$API_URL/api/knowledge/files")

if echo "$FILES_RESPONSE" | grep -q "\"id\":\"$FILE_ID_2\""; then
  echo -e "${RED}File still exists in registry after deletion${NC}"
  echo "Files response: $FILES_RESPONSE"
  exit 1
else
  echo -e "${GREEN}File successfully removed from registry${NC}"
fi

# Check if the agent's active file ID is now empty
echo -e "${BLUE}Checking if agent's active file ID is cleared...${NC}"
AGENTS_RESPONSE=$(curl -s "$API_URL/api/knowledge/agents")

if echo "$AGENTS_RESPONSE" | grep -q "\"id\":\"$AGENT_ID\"" && echo "$AGENTS_RESPONSE" | grep -q "\"active_knowledge_file_id\":\"\""; then
  echo -e "${GREEN}Agent's active file ID successfully cleared${NC}"
else
  echo -e "${RED}Agent's active file ID not cleared after deletion${NC}"
  echo "Agents response: $AGENTS_RESPONSE"
  exit 1
fi

echo -e "${GREEN}All tests passed successfully!${NC}"
