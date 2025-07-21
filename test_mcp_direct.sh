#!/bin/bash

# MCP Service URL
MCP_SERVICE_URL="https://bitwave-mcp-via-apis-service-455488113475.us-central1.run.app"

# Test MCP crypto price query (unauthenticated)
echo "Testing MCP crypto price query..."
curl -X GET "${MCP_SERVICE_URL}/api/v1/crypto/price?from_sym=BTC&to_fiat=USD" \
  -H "Content-Type: application/json"

echo -e "\n\n"

# Test MCP symbol info query (unauthenticated)
echo "Testing MCP symbol info query..."
curl -X GET "${MCP_SERVICE_URL}/api/v1/crypto/symbol?symbol=ETH" \
  -H "Content-Type: application/json"

echo -e "\n\n"
