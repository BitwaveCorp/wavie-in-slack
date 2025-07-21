# MCP Server API Usage Guide

This document provides practical examples for all available API endpoints in the MCP server, showing both the direct HTTP calls and the MCP request format.

## Base URL
```
https://bitwave-mcp-via-apis-service-455488113475.us-central1.run.app/
```

## 1. Get Cryptocurrency Price

### MCP Request Format
```bash
curl -X POST "https://bitwave-mcp-via-apis-service-455488113475.us-central1.run.app/" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "tools/call",
    "params": {
      "name": "get_crypto_price",
      "arguments": {
        "fromSym": "BTC",
        "toFiat": "USD",
        "timestampSEC": 1718888888,
        "service": "cryptocompare"
      }
    }
  }'
```

### Example Response
```json
{
  "result": {
    "close": { "mathjs": "BigNumber", "value": "65130.99" },
    "high": { "mathjs": "BigNumber", "value": "65982.45" },
    "low": { "mathjs": "BigNumber", "value": "65123.19" },
    "open": { "mathjs": "BigNumber", "value": "65968.42" },
    "price": { "mathjs": "BigNumber", "value": "65130.99" },
    "type": "candlestick",
    "volumeFrom": { "mathjs": "BigNumber", "value": "2398.93" },
    "volumeTo": { "mathjs": "BigNumber", "value": "157089063.76" }
  }
}
```

## 2. Lookup Symbol Information

### MCP Request Format
```bash
curl -X POST "https://bitwave-mcp-via-apis-service-455488113475.us-central1.run.app/" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "tools/call",
    "params": {
      "name": "lookup_symbol",
      "arguments": {
        "symbol": "BTC"
      }
    }
  }'
```

### Example Response
```json
{
  "result": {
    "symbol": "BTC",
    "name": "Bitcoin",
    "type": "cryptocurrency",
    "decimals": 8
  }
}
```

## 3. Get Authentication Token

### MCP Request Format
```bash
curl -X POST "https://bitwave-mcp-via-apis-service-455488113475.us-central1.run.app/" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "tools/call",
    "params": {
      "name": "get_token",
      "arguments": {
        "client_id": "your_client_id",
        "client_secret": "your_client_secret"
      }
    }
  }'
```

### Example Response
```json
{
  "result": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 3600,
    "token_type": "bearer"
  }
}
```

## 4. Get Organization Wallets

### MCP Request Format
```json
{
  "method": "tools/call",
  "params": {
    "name": "get_wallets",
    "arguments": {
      "orgId": "your-org-id"
    }
  }
}
```

### Response
```json
{
  "result": [
    {
      "id": "btc.testbtc.testbtc",
      "name": "test btc",
      "addresses": ["0x..."],
      "networkId": "btc",
      "orgId": "jtMvU6kEq7AF2DJIupb4",
      "createdSEC": 1724380000,
      "isSyncEnabled": true,
      "lastSuccessfulSyncSEC": 1753060000,
      "type": 1
    },
    {
      "id": "eth.testeth.testeth",
      "name": "test eth",
      "addresses": ["0x..."],
      "networkId": "eth",
      "orgId": "jtMvU6kEq7AF2DJIupb4",
      "createdSEC": 1724380000,
      "isSyncEnabled": true,
      "lastSuccessfulSyncSEC": 1753060000,
      "type": 1
    }
  ]
}
```

## 5. Get Organization Contacts

### MCP Request Format
```json
{
  "method": "tools/call",
  "params": {
    "name": "get_contacts",
    "arguments": {
      "orgId": "your-org-id"
    }
  }
}
```

### Response
```json
{
  "result": {
    "items": [
      {
        "id": "ctosfgR4Piovbn2zBWpI.1.11124",
        "name": "33Across",
        "type": "Customer",
        "firstName": "Yuna",
        "lastName": "Conn",
        "emailAddress": "",
        "enabled": true,
        "source": "SageIntacct"
      },
      {
        "id": "KmtjRcGxLxxoVpngRjHv.30",
        "name": "6529 Holdings LLC",
        "type": "Customer",
        "emailAddress": "jpower@6529.io",
        "enabled": true,
        "source": "QuickBooks",
        "addresses": [
          {
            "address": "0x8da5ff3999388b6c5d2a411c3f7f53f74c497037",
            "coin": "ETH",
            "memo": ""
          }
        ]
      }
    ],
    "nextPage": "pagination_token"
  }
}
```
```

## 6. Get Accounting Categories

### MCP Request Format
```json
{
  "method": "tools/call",
  "params": {
    "name": "get_categories",
    "arguments": {
      "orgId": "your-org-id"
    }
  }
}
```

### Response
```json
{
  "result": {
    "items": [
      {
        "id": "ctosfgR4Piovbn2zBWpI.5.155",
        "name": "AR - Retainage",
        "code": "12710",
        "type": "Asset",
        "source": "SageIntacct",
        "enabled": true,
        "lastUpdatedSEC": 1705937086
      },
      {
        "id": "KmtjRcGxLxxoVpngRjHv.53",
        "name": "ATM Sales",
        "code": "53",
        "type": "Revenue",
        "source": "QuickBooks",
        "enabled": true,
        "lastUpdatedSEC": 1753070905
      }
    ],
    "nextPage": "pagination_token"
  }
}
```

## 7. Get Organization Connections

### MCP Request Format
```json
{
  "method": "tools/call",
  "params": {
    "name": "get_connections",
    "arguments": {
      "orgId": "your-org-id"
    }
  }
}
```

### Response
```json
{
  "result": {
    "items": [
      {
        "id": "KmtjRcGxLxxoVpngRjHv",
        "provider": "QuickBooks",
        "type": "quickbooks",
        "name": "QuickBooks",
        "status": "OK",
        "isDefault": true,
        "isDisabled": false,
        "lastSyncSEC": 1753070911,
        "accountCode": "34",
        "feeAccountCode": "8"
      },
      {
        "id": "ZHpdyGRKyOR3ITkXSns8",
        "provider": "Binance",
        "type": "binance",
        "status": "OK",
        "isDisabled": false,
        "location": "standard"
      },
      {
        "id": "uym6TOyKgX78Csjxe0pT",
        "provider": "Xero",
        "type": "xero",
        "name": "Bitave-Local",
        "status": "Disabled",
        "isDisabled": true,
        "accountCode": "12",
        "feeAccountCode": "1"
      }
    ]
  }
}
```
  }'
```

## Notes
- Replace placeholder values like `your_client_id`, `your_client_secret`, and `your_organization_id` with actual values
- The `get_crypto_price` and `lookup_symbol` endpoints do not require authentication
- All other endpoints require a valid OAuth token in the `Authorization` header
- The MCP server expects all requests to be sent as JSON with the specified format
- Timestamps should be in Unix timestamp format (seconds since epoch)
