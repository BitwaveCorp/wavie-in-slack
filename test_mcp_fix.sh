#!/bin/bash

# Test script for MCP query handling after fix
# This script tests the MCP query handling without the initial processing message

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Testing MCP Query Handling After Fix${NC}"
echo "======================================"

# Test against local Claude Agent Proxy service
CLAUDE_PROXY_URL="http://localhost:8080"

# Test MCP crypto price query
echo -e "\n${YELLOW}Testing USDT price query:${NC}"
curl -s -X POST "${CLAUDE_PROXY_URL}/api/chat" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What is the price of USDT?",
    "user_id": "test_user",
    "channel_id": "test_channel",
    "message_ts": "123456789.123456",
    "thread_ts": "123456789.123456",
    "conversation_history": [],
    "correlation_id": "test_correlation_id"
  }' | jq '.'

echo -e "\n${YELLOW}Testing symbol info query:${NC}"
curl -s -X POST "${CLAUDE_PROXY_URL}/api/chat" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Tell me about the USDT token.",
    "user_id": "test_user",
    "channel_id": "test_channel",
    "message_ts": "123456789.123456",
    "thread_ts": "123456789.123456",
    "conversation_history": [],
    "correlation_id": "test_correlation_id"
  }' | jq '.'

echo -e "\n${YELLOW}Testing wallet query:${NC}"
curl -s -X POST "${CLAUDE_PROXY_URL}/api/chat" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Show me wallets with more than 1000 USDT.",
    "user_id": "test_user",
    "channel_id": "test_channel",
    "message_ts": "123456789.123456",
    "thread_ts": "123456789.123456",
    "conversation_history": [],
    "correlation_id": "test_correlation_id"
  }' | jq '.'

echo -e "\n${GREEN}Tests completed!${NC}"
