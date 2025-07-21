#!/bin/bash

# MCP Service URL
MCP_SERVICE_URL="https://bitwave-mcp-via-apis-service-455488113475.us-central1.run.app"

echo "Testing MCP service health check..."
curl -v "${MCP_SERVICE_URL}/health"

echo -e "\n\n"

echo "Testing MCP service API root..."
curl -v "${MCP_SERVICE_URL}/api"

echo -e "\n\n"

echo "Testing MCP crypto price endpoint..."
curl -v "${MCP_SERVICE_URL}/api/v1/crypto/price?from_sym=BTC&to_fiat=USD"

echo -e "\n\n"

echo "Testing MCP symbol lookup endpoint..."
curl -v "${MCP_SERVICE_URL}/api/v1/crypto/symbol?symbol=ETH"
