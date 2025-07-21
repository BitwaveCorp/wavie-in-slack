#!/bin/bash

# Check if the Slack events endpoint is available
echo "Testing Slack events endpoint..."
curl -v "https://wavie-claude-proxy-s-455488113475.us-central1.run.app/api/slack/events"

echo -e "\n\n"

# Check if there's a different chat completion endpoint
echo "Testing alternative chat completion endpoint..."
curl -v "https://wavie-claude-proxy-s-455488113475.us-central1.run.app/api/slack/chat"

echo -e "\n\n"

# Check the knowledge API endpoints we know exist
echo "Testing knowledge files API endpoint..."
curl -v "https://wavie-claude-proxy-s-455488113475.us-central1.run.app/api/knowledge/files"
