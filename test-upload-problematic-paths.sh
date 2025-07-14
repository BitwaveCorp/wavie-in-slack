#!/bin/bash

# Test script for uploading documents with problematic paths to the Claude Agent Proxy Service
# This script tests the upload functionality with paths containing characters not allowed in Firestore IDs

# Claude Agent Proxy Service URL from wavie-services.md
PROXY_URL="https://wavie-claude-proxy-s-455488113475.us-central1.run.app"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Testing Claude Agent Proxy Service Upload with Problematic Paths${NC}"
echo "============================================================="

# Create a test ZIP file with problematic paths
echo -e "\n${BLUE}Creating test ZIP file with problematic paths${NC}"

# Create temporary directory
TEMP_DIR=$(mktemp -d)
echo "Created temporary directory: $TEMP_DIR"

# Create files with problematic paths
mkdir -p "$TEMP_DIR/normal/path"
mkdir -p "$TEMP_DIR/path/with.dots"
mkdir -p "$TEMP_DIR/path/with[brackets]"
mkdir -p "$TEMP_DIR/path/with~tilde"
mkdir -p "$TEMP_DIR/path/with__double__underscores"

# Create test files
echo "This is a test file with normal path" > "$TEMP_DIR/normal/path/test1.txt"
echo "This is a test file with dots in path" > "$TEMP_DIR/path/with.dots/test2.txt"
echo "This is a test file with brackets in path" > "$TEMP_DIR/path/with[brackets]/test3.txt"
echo "This is a test file with tilde in path" > "$TEMP_DIR/path/with~tilde/test4.txt"
echo "This is a test file with double underscores in path" > "$TEMP_DIR/path/with__double__underscores/test5.txt"

# Create ZIP file
ZIP_FILE="$TEMP_DIR/test-problematic-paths.zip"
cd "$TEMP_DIR" && zip -r "test-problematic-paths.zip" normal path

echo -e "\n${BLUE}Created test ZIP file: $ZIP_FILE${NC}"

# Test uploading the ZIP file using curl
echo -e "\n${BLUE}Uploading test ZIP file to Claude Agent Proxy Service${NC}"
echo "This may take a moment..."

# Use curl to upload the file
RESPONSE=$(curl -s -X POST \
  -F "file=@$ZIP_FILE" \
  -F "name=test-problematic-paths" \
  -F "description=Test file with problematic paths" \
  -F "agent_ids=test-agent" \
  "$PROXY_URL/api/knowledge/upload")

echo -e "\n${BLUE}Upload Response:${NC}"
echo "$RESPONSE" | python -m json.tool

# Clean up
echo -e "\n${BLUE}Cleaning up temporary files${NC}"
rm -rf "$TEMP_DIR"

echo -e "\n${GREEN}Test completed!${NC}"
