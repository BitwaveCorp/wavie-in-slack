#!/bin/bash

# Test MCP crypto price query
echo "Testing MCP crypto price query..."
curl -X POST http://localhost:8083/api/chat/completion \
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
