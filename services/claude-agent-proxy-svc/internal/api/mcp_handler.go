package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc/internal/openai"
)

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getMapKeys returns a slice of all keys in a map[string]interface{}
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// detectMCPQuery checks if the message is an MCP query and extracts parameters
// using Claude's intelligence to determine the query type and extract parameters
func (h *Handler) detectMCPQuery(message string) (*MCPQuery, bool) {
	h.logger.Debug("Asking Claude to detect if message is an MCP query", "message", message)

	// Ask Claude to detect if this is an MCP query and extract parameters
	prompt := `You are Wavie, a digital assets accounting expert agent, whose primary expertise is in having full knowledge of the bitwave platform based on its user guide that's in your knowledge base. You are here to answer user questions, solve user problems and overall help them be successful using the bitwave platform. You have several features:

1. Answer general queries: 100% based on the knowledge base provided to you which is a full bitwave platform user guide / knowledge base
2. Answer balance queries: providing cryptocurrency wallet balance information upon request.
3. Get cryptocurrency prices: providing current or historical cryptocurrency prices.
4. Look up cryptocurrency symbol information: providing details about cryptocurrency symbols. More specifically check if bitwave supports a token
5. Get organization wallets: listing all wallets for a specific organization.
6. Get organization contacts: listing all contacts for a specific organization.
7. Get accounting categories: listing all accounting categories for a specific organization.
8. Get organization connections: listing all third-party connections for a specific organization.

You will receive a message and must determine if it is one of these types:
1. A cryptocurrency price query - A request to check a cryptocurrency price
2. A symbol information query - A request to check if bitwave supports or has a certain cryptocurrency symbol
3. An organization wallets query - A request to list wallets in the users organization
4. An organization contacts query - A request to list contacts in the users organization
5. An accounting categories query - A request to list accounting categories in the users organization
6. An organization connections query - A request to list connections in the users organization
7. A balance query - A request to check a cryptocurrency wallet address balance
8. A general query - Any other type of message related to the bitwave platform

For cryptocurrency price queries, you MUST respond with a JSON object containing these fields:
- type: Always "mcp_query"
- query_type: Always "crypto_price"
- from_sym: The cryptocurrency symbol (e.g., "BTC", "ETH")
- to_fiat: The fiat currency (e.g., "USD", "EUR")
- timestamp_sec: (Optional) The timestamp in seconds since epoch for historical prices
- service: (Optional) The price service to use (e.g., "cryptocompare")

For symbol information queries, you MUST respond with a JSON object containing these fields:
- type: Always "mcp_query"
- query_type: Always "symbol_info"
- symbol: The cryptocurrency symbol (e.g., "BTC", "ETH")

For organization wallets queries, you MUST respond with a JSON object containing these fields:
- type: Always "mcp_query"
- query_type: Always "wallets"
- client_id: The client ID for authentication
- client_secret: The client secret for authentication
- org_id: The organization ID

For organization contacts queries, you MUST respond with a JSON object containing these fields:
- type: Always "mcp_query"
- query_type: Always "contacts"
- client_id: The client ID for authentication
- client_secret: The client secret for authentication
- org_id: The organization ID

For accounting categories queries, you MUST respond with a JSON object containing these fields:
- type: Always "mcp_query"
- query_type: Always "categories"
- client_id: The client ID for authentication
- client_secret: The client secret for authentication
- org_id: The organization ID

For organization connections queries, you MUST respond with a JSON object containing these fields:
- type: Always "mcp_query"
- query_type: Always "connections"
- client_id: The client ID for authentication
- client_secret: The client secret for authentication
- org_id: The organization ID

For balance queries, you MUST respond with a JSON object containing these fields:
- type: Always "balance_query"
- chain: The blockchain network (e.g., "ethereum", "bitcoin", "polygon")
- address: The wallet address
- token_contract: (Optional) The token contract address if it's a token balance query

For general queries, respond with an empty JSON object: {}

Examples of MCP queries:
- "What's the current price of Bitcoin in USD?"
  {"type":"mcp_query","query_type":"crypto_price","from_sym":"BTC","to_fiat":"USD"}
- "What was the price of Ethereum yesterday?"
  {"type":"mcp_query","query_type":"crypto_price","from_sym":"ETH","to_fiat":"USD","timestamp_sec":1718888888}
- "Does Bitwave have BTC token support?"
  {"type":"mcp_query","query_type":"symbol_info","symbol":"BTC"}
- "Show me all wallets in my org"
  {"type":"mcp_query","query_type":"wallets","client_id":"YOUR_CLIENT_ID","client_secret":"YOUR_CLIENT_SECRET","org_id":"ABC123"}
- "Show me all contacts in my org"
  {"type":"mcp_query","query_type":"contacts","client_id":"YOUR_CLIENT_ID","client_secret":"YOUR_CLIENT_SECRET","org_id":"ABC123"}
- "Show me all accounting categories in my org"
  {"type":"mcp_query","query_type":"categories","client_id":"YOUR_CLIENT_ID","client_secret":"YOUR_CLIENT_SECRET","org_id":"ABC123"}
- "Show me all connections in my org"
  {"type":"mcp_query","query_type":"connections","client_id":"YOUR_CLIENT_ID","client_secret":"YOUR_CLIENT_SECRET","org_id":"ABC123"}

Now analyze this message (respond with only the JSON object, no other text or formatting):
` + message

	// Create a background context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	h.logger.Debug("Sending message to Claude for MCP query detection")

	// Call Claude with the prompt
	resp, err := h.openaiClient.CreateChatCompletion(ctx, []openai.ChatMessage{
		{
			Role:    "system",
			Content: "You are a classifier that determines if a message is requesting cryptocurrency or accounting data. Respond with only a JSON object, no other text or formatting. IMPORTANT: Slack user IDs (like U0916GH1GBY) are NOT client IDs or credentials - never interpret them as such.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}, 0.0) // Very low temperature for deterministic output

	if err != nil {
		h.logger.Error("Failed to call Claude for MCP query detection", "error", err)
		return nil, false
	}

	h.logger.Debug("Received response from Claude", "response", resp)

	// Clean the response to extract just the JSON part
	jsonStart := strings.Index(resp, "{")
	jsonEnd := strings.LastIndex(resp, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		h.logger.Debug("No valid JSON object found in Claude's response")
		return nil, false
	}

	jsonStr := resp[jsonStart : jsonEnd+1]

	// Parse the response
	var query MCPQuery
	if err := json.Unmarshal([]byte(jsonStr), &query); err != nil {
		h.logger.Error("Failed to parse Claude's response as JSON", "error", err, "json", jsonStr)
		return nil, false
	}

	// If type is not mcp_query, this is not an MCP query
	if query.Type != "mcp_query" {
		h.logger.Debug("Claude determined this is not an MCP query")
		return nil, false
	}

	// Log the detected query
	h.logger.Info("Claude detected an MCP query",
		"query_type", query.QueryType)

	return &query, true
}

// handleMCPQuery processes an MCP query
func (h *Handler) handleMCPQuery(w http.ResponseWriter, r *http.Request, query *MCPQuery, correlationID string) {
	h.logger.Info("Processing MCP query",
		"query_type", query.QueryType,
		"correlation_id", correlationID)

	// Set content type for the response
	w.Header().Set("Content-Type", "application/json")

	// Process the query based on the query type
	var result map[string]interface{}
	var err error

	switch query.QueryType {
	case string(MCPQueryCryptoPrice):
		result, err = h.handleCryptoPriceQuery(r.Context(), query, correlationID)
	case string(MCPQuerySymbolInfo):
		result, err = h.handleSymbolInfoQuery(r.Context(), query, correlationID)
	case string(MCPQueryWallets):
		result, err = h.handleWalletsQuery(r.Context(), query, correlationID)
	case string(MCPQueryContacts):
		result, err = h.handleContactsQuery(r.Context(), query, correlationID)
	case string(MCPQueryCategories):
		result, err = h.handleCategoriesQuery(r.Context(), query, correlationID)
	case string(MCPQueryConnections):
		result, err = h.handleConnectionsQuery(r.Context(), query, correlationID)
	default:
		err = fmt.Errorf("unsupported MCP query type: %s", query.QueryType)
	}

	if err != nil {
		h.logger.Error("Failed to process MCP query",
			"error", err,
			"correlation_id", correlationID,
			"query_type", query.QueryType,
		)

		// Format error response
		errResponse := ChatResponse{
			Response:      fmt.Sprintf("❌ I couldn't process your request: %s", err.Error()),
			CorrelationID: correlationID,
		}

		if err := json.NewEncoder(w).Encode(errResponse); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	// Format the response based on the query type
	var response string
	var responseData map[string]interface{}

	switch query.QueryType {
	case string(MCPQueryCryptoPrice):
		response, responseData = h.formatCryptoPriceResponse(query, result)
	case string(MCPQuerySymbolInfo):
		response, responseData = h.formatSymbolInfoResponse(query, result)
	case string(MCPQueryWallets):
		response, responseData = h.formatWalletsResponse(query, result)
	case string(MCPQueryContacts):
		response, responseData = h.formatContactsResponse(query, result)
	case string(MCPQueryCategories):
		response, responseData = h.formatCategoriesResponse(query, result)
	case string(MCPQueryConnections):
		response, responseData = h.formatConnectionsResponse(query, result)
	}

	// Create the response with MCP data and flags to prevent broadcasting
	jsonResponse := map[string]interface{}{
		"response":        response,
		"correlation_id":  correlationID,
		"should_broadcast": false,
		"response_type":   "mcp",
		"data":            responseData,
	}

	// Send the response
	if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
		h.logger.Error("Failed to encode MCP response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleCryptoPriceQuery handles a cryptocurrency price query
func (h *Handler) handleCryptoPriceQuery(ctx context.Context, query *MCPQuery, correlationID string) (map[string]interface{}, error) {
	h.logger.Info("Processing crypto price query",
		"from_sym", query.FromSym,
		"to_fiat", query.ToFiat,
		"timestamp_sec", query.TimestampSEC,
		"correlation_id", correlationID)

	// Check if required parameters are missing
	if query.FromSym == "" {
		return nil, fmt.Errorf("missing cryptocurrency symbol. Please provide the cryptocurrency symbol in your request. Example: 'What is the price of BTC in USD?'")
	}

	if query.ToFiat == "" {
		return nil, fmt.Errorf("missing fiat currency. Please provide the fiat currency in your request. Example: 'What is the price of BTC in USD?'")
	}

	// Set default values if not provided
	if query.Service == "" {
		query.Service = "cryptocompare"
	}

	// If timestamp is not provided, use current time
	if query.TimestampSEC == 0 {
		query.TimestampSEC = time.Now().Unix()
	}

	// Get the price from the MCP service
	result, err := h.mcpSvc.GetCryptoPrice(ctx, query.FromSym, query.ToFiat, query.TimestampSEC, query.Service)
	if err != nil {
		return nil, fmt.Errorf("failed to get crypto price: %v", err)
	}

	return result, nil
}

// handleSymbolInfoQuery handles a symbol information query
func (h *Handler) handleSymbolInfoQuery(ctx context.Context, query *MCPQuery, correlationID string) (map[string]interface{}, error) {
	h.logger.Info("Processing symbol info query",
		"symbol", query.Symbol,
		"correlation_id", correlationID)

	// Check if required parameters are missing
	if query.Symbol == "" {
		return nil, fmt.Errorf("missing cryptocurrency symbol. Please provide the cryptocurrency symbol in your request. Example: 'Tell me about the USDT token' or 'Does Bitwave support BTC?'")
	}

	// Get the symbol information from the MCP service
	result, err := h.mcpSvc.LookupSymbol(ctx, query.Symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbol information: %v", err)
	}

	return result, nil
}

// handleWalletsQuery handles an organization wallets query
func (h *Handler) handleWalletsQuery(ctx context.Context, query *MCPQuery, correlationID string) (map[string]interface{}, error) {
	h.logger.Info("Processing wallets query",
		"org_id", query.OrgID,
		"correlation_id", correlationID)

	// Check if credentials are missing
	if query.ClientID == "" || query.ClientSecret == "" {
		return nil, fmt.Errorf("missing credentials. Please provide your client_id and client_secret in your request. Example: 'Show me all wallets in my org with client_id=abc123 and client_secret=xyz789'")
	}

	// Check if org_id is missing
	if query.OrgID == "" {
		return nil, fmt.Errorf("missing organization ID. Please provide your org_id in your request. Example: 'Show me all wallets in my org with client_id=abc123, client_secret=xyz789, and org_id=org123'")
	}

	// Get authentication token
	authResp, err := h.mcpSvc.GetToken(ctx, query.ClientID, query.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to get authentication token: %v", err)
	}

	// Get the wallets from the MCP service
	result, err := h.mcpSvc.GetWallets(ctx, query.OrgID, authResp.Result.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallets: %v", err)
	}

	// Debug log the raw response structure
	resultJSON, _ := json.Marshal(result)
	h.logger.Info("Raw wallets response", 
		"result_type", fmt.Sprintf("%T", result),
		"has_items", result["items"] != nil,
		"has_result", result["result"] != nil,
		"response_preview", string(resultJSON)[:min(200, len(string(resultJSON)))], // Log first 200 chars
		"correlation_id", correlationID)

	return result, nil
}

// handleContactsQuery handles an organization contacts query
func (h *Handler) handleContactsQuery(ctx context.Context, query *MCPQuery, correlationID string) (map[string]interface{}, error) {
	h.logger.Info("Processing contacts query",
		"org_id", query.OrgID,
		"correlation_id", correlationID)

	// Check if credentials are missing
	if query.ClientID == "" || query.ClientSecret == "" {
		return nil, fmt.Errorf("missing credentials. Please provide your client_id and client_secret in your request. Example: 'Show me all contacts in my org with client_id=abc123 and client_secret=xyz789'")
	}

	// Check if org_id is missing
	if query.OrgID == "" {
		return nil, fmt.Errorf("missing organization ID. Please provide your org_id in your request. Example: 'Show me all contacts in my org with client_id=abc123, client_secret=xyz789, and org_id=org123'")
	}

	// Get authentication token
	authResp, err := h.mcpSvc.GetToken(ctx, query.ClientID, query.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to get authentication token: %v", err)
	}

	// Get the contacts from the MCP service
	result, err := h.mcpSvc.GetContacts(ctx, query.OrgID, authResp.Result.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get contacts: %v", err)
	}

	return result, nil
}

// handleCategoriesQuery handles an accounting categories query
func (h *Handler) handleCategoriesQuery(ctx context.Context, query *MCPQuery, correlationID string) (map[string]interface{}, error) {
	h.logger.Info("Processing categories query",
		"org_id", query.OrgID,
		"correlation_id", correlationID)

	// Check if credentials are missing
	if query.ClientID == "" || query.ClientSecret == "" {
		return nil, fmt.Errorf("missing credentials. Please provide your client_id and client_secret in your request. Example: 'Show me all accounting categories in my org with client_id=abc123 and client_secret=xyz789'")
	}

	// Check if org_id is missing
	if query.OrgID == "" {
		return nil, fmt.Errorf("missing organization ID. Please provide your org_id in your request. Example: 'Show me all accounting categories in my org with client_id=abc123, client_secret=xyz789, and org_id=org123'")
	}

	// Get authentication token
	authResp, err := h.mcpSvc.GetToken(ctx, query.ClientID, query.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to get authentication token: %v", err)
	}

	// Get the categories from the MCP service
	result, err := h.mcpSvc.GetCategories(ctx, query.OrgID, authResp.Result.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %v", err)
	}

	return result, nil
}

// handleConnectionsQuery handles an organization connections query
func (h *Handler) handleConnectionsQuery(ctx context.Context, query *MCPQuery, correlationID string) (map[string]interface{}, error) {
	h.logger.Info("Processing connections query",
		"org_id", query.OrgID,
		"correlation_id", correlationID)

	// Check if credentials are missing
	if query.ClientID == "" || query.ClientSecret == "" {
		return nil, fmt.Errorf("missing credentials. Please provide your client_id and client_secret in your request. Example: 'Show me all connections in my org with client_id=abc123 and client_secret=xyz789'")
	}

	// Check if org_id is missing
	if query.OrgID == "" {
		return nil, fmt.Errorf("missing organization ID. Please provide your org_id in your request. Example: 'Show me all connections in my org with client_id=abc123, client_secret=xyz789, and org_id=org123'")
	}

	// Get authentication token
	authResp, err := h.mcpSvc.GetToken(ctx, query.ClientID, query.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to get authentication token: %v", err)
	}

	// Get the connections from the MCP service
	result, err := h.mcpSvc.GetConnections(ctx, query.OrgID, authResp.Result.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get connections: %v", err)
	}

	return result, nil
}

// formatCryptoPriceResponse formats the response for a cryptocurrency price query
func (h *Handler) formatCryptoPriceResponse(query *MCPQuery, result map[string]interface{}) (string, map[string]interface{}) {
	// Extract the price data
	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return "❌ Failed to parse price data", nil
	}

	// Extract the price value
	var price string
	if priceData, ok := resultData["price"].(map[string]interface{}); ok {
		if value, ok := priceData["value"].(string); ok {
			price = value
		}
	}

	// Format the response
	response := fmt.Sprintf("💰 *%s/%s Price*\n\n"+
		"*Price:* %s %s\n"+
		"*Timestamp:* %s",
		strings.ToUpper(query.FromSym),
		strings.ToUpper(query.ToFiat),
		price,
		strings.ToUpper(query.ToFiat),
		time.Unix(query.TimestampSEC, 0).Format("January 2, 2006 15:04:05 MST"),
	)

	responseData := map[string]interface{}{
		"type":          "crypto_price",
		"from_sym":      query.FromSym,
		"to_fiat":       query.ToFiat,
		"price":         price,
		"timestamp_sec": query.TimestampSEC,
		"formatted":     response,
	}

	return response, responseData
}

// formatSymbolInfoResponse formats the response for a symbol information query
func (h *Handler) formatSymbolInfoResponse(query *MCPQuery, result map[string]interface{}) (string, map[string]interface{}) {
	// Extract the symbol data
	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return "❌ Failed to parse symbol data", nil
	}

	// Extract the symbol information
	symbol := fmt.Sprintf("%v", resultData["symbol"])
	name := fmt.Sprintf("%v", resultData["name"])
	symbolType := fmt.Sprintf("%v", resultData["type"])
	decimals := fmt.Sprintf("%v", resultData["decimals"])

	// Format the response
	response := fmt.Sprintf("ℹ️ *%s Symbol Information*\n\n"+
		"*Symbol:* %s\n"+
		"*Name:* %s\n"+
		"*Type:* %s\n"+
		"*Decimals:* %s",
		strings.ToUpper(symbol),
		symbol,
		name,
		symbolType,
		decimals,
	)

	responseData := map[string]interface{}{
		"type":        "symbol_info",
		"symbol":      symbol,
		"name":        name,
		"symbol_type": symbolType,
		"decimals":    decimals,
		"formatted":   response,
	}

	return response, responseData
}

// formatWalletsResponse formats the response for an organization wallets query
func (h *Handler) formatWalletsResponse(query *MCPQuery, result map[string]interface{}) (string, map[string]interface{}) {
	// Log the raw result for debugging
	resultJSON, _ := json.Marshal(result)
	h.logger.Info("Formatting wallets response", 
		"result_keys", getMapKeys(result),
		"result_length", len(resultJSON))

	// Extract the wallets data using multiple approaches
	var resultData []interface{}
	var found bool
	
	// Approach 1: Try to get items array first (actual structure from API)
	if items, hasItems := result["items"].([]interface{}); hasItems && len(items) > 0 {
		h.logger.Info("Found items array in response", "items_count", len(items))
		resultData = items
		found = true
	} else if res, hasResult := result["result"].([]interface{}); hasResult && len(res) > 0 {
		// Approach 2: Fallback to result array (previous expected structure)
		h.logger.Info("Found result array in response", "result_count", len(res))
		resultData = res
		found = true
	} else if resultMap, isMap := result["result"].(map[string]interface{}); isMap {
		// Approach 3: Check if result is a map that contains items
		if items, hasItems := resultMap["items"].([]interface{}); hasItems && len(items) > 0 {
			h.logger.Info("Found items array in result map", "items_count", len(items))
			resultData = items
			found = true
		}
	} else {
		// Approach 4: Last resort - try to find any array in the response
		for key, value := range result {
			if arr, isArray := value.([]interface{}); isArray && len(arr) > 0 {
				h.logger.Info("Found array in response under key", "key", key, "count", len(arr))
				resultData = arr
				found = true
				break
			}
		}
	}
	
	// If we still couldn't find any wallet data
	if !found || len(resultData) == 0 {
		// Log detailed information about the result structure
		h.logger.Error("Failed to parse wallets data", 
			"result_type", fmt.Sprintf("%T", result),
			"result_keys", getMapKeys(result),
			"raw_result", string(resultJSON)[:min(500, len(string(resultJSON)))])
		return "❌ Failed to parse wallets data. The response structure was unexpected.", nil
	}

	// Format the response
	var walletsList strings.Builder
	walletCount := len(resultData)

	walletsList.WriteString(fmt.Sprintf("📒 *Organization Wallets (%d)*\n\n", walletCount))

	// Add each wallet to the list
	for i, wallet := range resultData {
		if i >= 10 {
			walletsList.WriteString(fmt.Sprintf("\n...and %d more wallets", walletCount-10))
			break
		}

		walletData, ok := wallet.(map[string]interface{})
		if !ok {
			continue
		}

		name := fmt.Sprintf("%v", walletData["name"])
		id := fmt.Sprintf("%v", walletData["id"])
		
		// Handle different wallet types and structures
		walletsList.WriteString(fmt.Sprintf("*%d. %s*\n", i+1, name))
		walletsList.WriteString(fmt.Sprintf("   ID: %s\n", id))
		
		// Add network ID if available
		if networkID, hasNetwork := walletData["networkId"]; hasNetwork && networkID != nil && networkID != "" {
			walletsList.WriteString(fmt.Sprintf("   Network: %v\n", networkID))
		}
		
		// Add wallet type if available
		if walletType, hasType := walletData["type"]; hasType {
			walletsList.WriteString(fmt.Sprintf("   Type: %v\n", walletType))
		}
		
		// Add addresses if available
		if addresses, hasAddresses := walletData["addresses"].([]interface{}); hasAddresses && len(addresses) > 0 {
			walletsList.WriteString("   Addresses:\n")
			for j, addr := range addresses {
				if j >= 2 { // Limit to 2 addresses per wallet
					walletsList.WriteString(fmt.Sprintf("      ...and %d more\n", len(addresses)-2))
					break
				}
				walletsList.WriteString(fmt.Sprintf("      - %v\n", addr))
			}
		}
		
		walletsList.WriteString("\n")
	}

	responseData := map[string]interface{}{
		"type":      "wallets",
		"org_id":    query.OrgID,
		"count":     walletCount,
		"wallets":   resultData,
		"formatted": walletsList.String(),
	}

	return walletsList.String(), responseData
}

// formatContactsResponse formats the response for an organization contacts query
func (h *Handler) formatContactsResponse(query *MCPQuery, result map[string]interface{}) (string, map[string]interface{}) {
	// Extract the contacts data
	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return "❌ Failed to parse contacts data", nil
	}

	items, ok := resultData["items"].([]interface{})
	if !ok {
		return "❌ Failed to parse contacts items", nil
	}

	// Format the response
	var contactsList strings.Builder
	contactCount := len(items)

	contactsList.WriteString(fmt.Sprintf("👥 *Organization Contacts (%d)*\n\n", contactCount))

	// Add each contact to the list
	for i, contact := range items {
		if i >= 10 {
			contactsList.WriteString(fmt.Sprintf("\n...and %d more contacts", contactCount-10))
			break
		}

		contactData, ok := contact.(map[string]interface{})
		if !ok {
			continue
		}

		name := fmt.Sprintf("%v", contactData["name"])
		contactType := fmt.Sprintf("%v", contactData["type"])
		email := fmt.Sprintf("%v", contactData["emailAddress"])

		contactsList.WriteString(fmt.Sprintf("*%d. %s*\n", i+1, name))
		contactsList.WriteString(fmt.Sprintf("   Type: %s\n", contactType))
		if email != "" && email != "<nil>" {
			contactsList.WriteString(fmt.Sprintf("   Email: %s\n", email))
		}
		contactsList.WriteString("\n")
	}

	// Check if there's pagination
	nextPage, hasNextPage := resultData["nextPage"].(string)

	if hasNextPage && nextPage != "" {
		contactsList.WriteString("\n_Additional contacts available. Please refine your query to see more._")
	}

	responseData := map[string]interface{}{
		"type":      "contacts",
		"org_id":    query.OrgID,
		"count":     contactCount,
		"contacts":  items,
		"has_more":  hasNextPage,
		"formatted": contactsList.String(),
	}

	return contactsList.String(), responseData
}

// formatCategoriesResponse formats the response for an accounting categories query
func (h *Handler) formatCategoriesResponse(query *MCPQuery, result map[string]interface{}) (string, map[string]interface{}) {
	// Extract the categories data
	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return "❌ Failed to parse categories data", nil
	}

	items, ok := resultData["items"].([]interface{})
	if !ok {
		return "❌ Failed to parse categories items", nil
	}

	// Format the response
	var categoriesList strings.Builder
	categoryCount := len(items)

	categoriesList.WriteString(fmt.Sprintf("📊 *Accounting Categories (%d)*\n\n", categoryCount))

	// Add each category to the list
	for i, category := range items {
		if i >= 10 {
			categoriesList.WriteString(fmt.Sprintf("\n...and %d more categories", categoryCount-10))
			break
		}

		categoryData, ok := category.(map[string]interface{})
		if !ok {
			continue
		}

		name := fmt.Sprintf("%v", categoryData["name"])
		code := fmt.Sprintf("%v", categoryData["code"])
		categoryType := fmt.Sprintf("%v", categoryData["type"])
		source := fmt.Sprintf("%v", categoryData["source"])

		categoriesList.WriteString(fmt.Sprintf("*%d. %s*\n", i+1, name))
		categoriesList.WriteString(fmt.Sprintf("   Code: %s\n", code))
		categoriesList.WriteString(fmt.Sprintf("   Type: %s\n", categoryType))
		categoriesList.WriteString(fmt.Sprintf("   Source: %s\n\n", source))
	}

	// Check if there's pagination
	nextPage, hasNextPage := resultData["nextPage"].(string)

	if hasNextPage && nextPage != "" {
		categoriesList.WriteString("\n_Additional categories available. Please refine your query to see more._")
	}

	responseData := map[string]interface{}{
		"type":       "categories",
		"org_id":     query.OrgID,
		"count":      categoryCount,
		"categories": items,
		"has_more":   hasNextPage,
		"formatted":  categoriesList.String(),
	}

	return categoriesList.String(), responseData
}

// formatConnectionsResponse formats the response for an organization connections query
func (h *Handler) formatConnectionsResponse(query *MCPQuery, result map[string]interface{}) (string, map[string]interface{}) {
	// Extract the connections data
	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return "❌ Failed to parse connections data", nil
	}

	items, ok := resultData["items"].([]interface{})
	if !ok {
		return "❌ Failed to parse connections items", nil
	}

	// Format the response
	var connectionsList strings.Builder
	connectionCount := len(items)

	connectionsList.WriteString(fmt.Sprintf("🔌 *Organization Connections (%d)*\n\n", connectionCount))

	// Add each connection to the list
	for i, connection := range items {
		if i >= 10 {
			connectionsList.WriteString(fmt.Sprintf("\n...and %d more connections", connectionCount-10))
			break
		}

		connectionData, ok := connection.(map[string]interface{})
		if !ok {
			continue
		}

		provider := fmt.Sprintf("%v", connectionData["provider"])
		connectionType := fmt.Sprintf("%v", connectionData["type"])
		status := fmt.Sprintf("%v", connectionData["status"])

		// Check if the connection is disabled
		isDisabled, ok := connectionData["isDisabled"].(bool)

		// Format the name - some connections might not have a name
		name := ""
		if nameVal, ok := connectionData["name"]; ok && nameVal != nil {
			name = fmt.Sprintf("%v", nameVal)
		}

		// Display provider as the name if no name is available
		displayName := provider
		if name != "" && name != "<nil>" {
			displayName = name
		}

		connectionsList.WriteString(fmt.Sprintf("*%d. %s*\n", i+1, displayName))
		connectionsList.WriteString(fmt.Sprintf("   Provider: %s\n", provider))
		connectionsList.WriteString(fmt.Sprintf("   Type: %s\n", connectionType))

		// Format the status with an emoji indicator
		statusEmoji := "✅"
		if isDisabled || strings.Contains(strings.ToLower(status), "disabled") {
			statusEmoji = "❌"
		} else if strings.Contains(strings.ToLower(status), "error") {
			statusEmoji = "⚠️"
		}

		connectionsList.WriteString(fmt.Sprintf("   Status: %s %s\n\n", statusEmoji, status))
	}

	responseData := map[string]interface{}{
		"type":        "connections",
		"org_id":      query.OrgID,
		"count":       connectionCount,
		"connections": items,
		"formatted":   connectionsList.String(),
	}

	return connectionsList.String(), responseData
}
