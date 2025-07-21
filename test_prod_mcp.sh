#!/bin/bash

# Production Claude Agent Proxy Service URL
CLAUDE_PROXY_URL="https://wavie-claude-proxy-s-455488113475.us-central1.run.app"

# Test MCP crypto price query
echo "Testing MCP crypto price query..."
curl -X POST "${CLAUDE_PROXY_URL}/api/chat/completion" \
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

# Test MCP symbol info query
echo "Testing MCP symbol info query..."
curl -X POST "${CLAUDE_PROXY_URL}/api/chat/completion" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Does Bitwave support ETH token?",
    "user_id": "test_user",
    "channel_id": "test_channel",
    "thread_ts": "",
    "message_ts": "",
    "conversation_history": []
  }'

echo -e "\n\n"

# Test balance query
echo "Testing balance query..."
curl -X POST "${CLAUDE_PROXY_URL}/api/chat/completion" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What is the balance of 0x742d35Cc6634C0532925a3b844Bc454e4438f44e on Ethereum?",
    "user_id": "test_user",
    "channel_id": "test_channel",
    "thread_ts": "",
    "message_ts": "",
    "conversation_history": []
  }'
