#!/bin/bash

# Production Claude Agent Proxy Service URL
CLAUDE_PROXY_URL="https://wavie-claude-proxy-s-455488113475.us-central1.run.app"

# Test MCP crypto price query with verbose output
echo "Testing MCP crypto price query..."
curl -v -X POST "${CLAUDE_PROXY_URL}/api/chat/completion" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What is the current price of Bitcoin in USD?",
    "user_id": "test_user",
    "channel_id": "test_channel",
    "thread_ts": "",
    "message_ts": "",
    "conversation_history": []
  }'

echo -e "\n\n"

# Test direct access to the service root to check if it's accessible
echo "Testing service root access..."
curl -v "${CLAUDE_PROXY_URL}/"

echo -e "\n\n"

# Test direct access to the MCP service health endpoint
echo "Testing MCP service health..."
curl -v "https://bitwave-mcp-via-apis-service-455488113475.us-central1.run.app/health"
