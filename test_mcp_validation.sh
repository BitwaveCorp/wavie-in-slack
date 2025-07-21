#!/bin/bash

# Test script for MCP query parameter validation
# This script tests the improved parameter validation for MCP queries

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Testing MCP Query Parameter Validation${NC}"
echo "======================================"

# Test against local Claude Agent Proxy service
CLAUDE_PROXY_URL="http://localhost:8080"

# Test missing parameters for crypto price query
echo -e "\n${YELLOW}Testing crypto price query with missing parameters:${NC}"
curl -s -X POST "${CLAUDE_PROXY_URL}/api/chat" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What is the price of crypto?",
    "user_id": "test_user",
    "channel_id": "test_channel",
    "message_ts": "123456789.123456",
    "thread_ts": "123456789.123456",
    "conversation_history": [],
    "correlation_id": "test_correlation_id"
  }' | jq '.'

# Test missing parameters for symbol info query
echo -e "\n${YELLOW}Testing symbol info query with missing parameters:${NC}"
curl -s -X POST "${CLAUDE_PROXY_URL}/api/chat" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Tell me about the token.",
    "user_id": "test_user",
    "channel_id": "test_channel",
    "message_ts": "123456789.123456",
    "thread_ts": "123456789.123456",
    "conversation_history": [],
    "correlation_id": "test_correlation_id"
  }' | jq '.'

# Test missing credentials for wallets query
echo -e "\n${YELLOW}Testing wallets query with missing credentials:${NC}"
curl -s -X POST "${CLAUDE_PROXY_URL}/api/chat" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Show me wallets in my org.",
    "user_id": "test_user",
    "channel_id": "test_channel",
    "message_ts": "123456789.123456",
    "thread_ts": "123456789.123456",
    "conversation_history": [],
    "correlation_id": "test_correlation_id"
  }' | jq '.'

# Test missing org_id for contacts query
echo -e "\n${YELLOW}Testing contacts query with missing org_id:${NC}"
curl -s -X POST "${CLAUDE_PROXY_URL}/api/chat" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Show me contacts with client_id=abc123 and client_secret=xyz789.",
    "user_id": "test_user",
    "channel_id": "test_channel",
    "message_ts": "123456789.123456",
    "thread_ts": "123456789.123456",
    "conversation_history": [],
    "correlation_id": "test_correlation_id"
  }' | jq '.'

echo -e "\n${GREEN}Tests completed!${NC}"
