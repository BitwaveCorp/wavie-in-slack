package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Client represents an MCP service client
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new MCP client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // 60 second timeout as specified
		},
	}
}

// AuthResponse represents the authentication response from the MCP service
type AuthResponse struct {
	Result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	} `json:"result"`
}

// MCPRequest represents a request to the MCP service
type MCPRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// GetCryptoPrice gets the price of a cryptocurrency
func (c *Client) GetCryptoPrice(ctx context.Context, fromSym, toFiat string, timestampSEC int64, service string) (map[string]interface{}, error) {
	log.Printf("Getting crypto price for %s/%s at timestamp %d using service %s", fromSym, toFiat, timestampSEC, service)

	// Create the request
	req := MCPRequest{
		Method: "tools/call",
		Params: map[string]interface{}{
			"name": "get_crypto_price",
			"arguments": map[string]interface{}{
				"fromSym":      fromSym,
				"toFiat":       toFiat,
				"timestampSEC": timestampSEC,
				"service":      service,
			},
		},
	}

	// Make the request
	return c.makeRequest(ctx, req)
}

// LookupSymbol looks up information about a cryptocurrency symbol
func (c *Client) LookupSymbol(ctx context.Context, symbol string) (map[string]interface{}, error) {
	log.Printf("Looking up symbol information for %s", symbol)

	// Create the request
	req := MCPRequest{
		Method: "tools/call",
		Params: map[string]interface{}{
			"name": "lookup_symbol",
			"arguments": map[string]interface{}{
				"symbol": symbol,
			},
		},
	}

	// Make the request
	return c.makeRequest(ctx, req)
}

// GetToken gets an authentication token
func (c *Client) GetToken(ctx context.Context, clientID, clientSecret string) (*AuthResponse, error) {
	log.Printf("Getting authentication token for client ID %s", clientID)

	// Create the request
	req := MCPRequest{
		Method: "tools/call",
		Params: map[string]interface{}{
			"name": "get_token",
			"arguments": map[string]interface{}{
				"client_id":     clientID,
				"client_secret": clientSecret,
			},
		},
	}

	// Make the request
	respData, err := c.makeRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert to AuthResponse
	respBytes, err := json.Marshal(respData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %v", err)
	}

	var authResp AuthResponse
	if err := json.Unmarshal(respBytes, &authResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth response: %v", err)
	}

	return &authResp, nil
}

// GetWallets gets the wallets for an organization
func (c *Client) GetWallets(ctx context.Context, orgID, accessToken string) (map[string]interface{}, error) {
	log.Printf("Getting wallets for organization %s", orgID)

	// Create the request
	req := MCPRequest{
		Method: "tools/call",
		Params: map[string]interface{}{
			"name": "get_wallets",
			"arguments": map[string]interface{}{
				"orgId": orgID,
			},
		},
	}

	// Make the request with authentication
	return c.makeAuthenticatedRequest(ctx, req, accessToken)
}

// GetContacts gets the contacts for an organization
func (c *Client) GetContacts(ctx context.Context, orgID, accessToken string) (map[string]interface{}, error) {
	log.Printf("Getting contacts for organization %s", orgID)

	// Create the request
	req := MCPRequest{
		Method: "tools/call",
		Params: map[string]interface{}{
			"name": "get_contacts",
			"arguments": map[string]interface{}{
				"orgId": orgID,
			},
		},
	}

	// Make the request with authentication
	return c.makeAuthenticatedRequest(ctx, req, accessToken)
}

// GetCategories gets the accounting categories for an organization
func (c *Client) GetCategories(ctx context.Context, orgID, accessToken string) (map[string]interface{}, error) {
	log.Printf("Getting accounting categories for organization %s", orgID)

	// Create the request
	req := MCPRequest{
		Method: "tools/call",
		Params: map[string]interface{}{
			"name": "get_categories",
			"arguments": map[string]interface{}{
				"orgId": orgID,
			},
		},
	}

	// Make the request with authentication
	return c.makeAuthenticatedRequest(ctx, req, accessToken)
}

// GetConnections gets the connections for an organization
func (c *Client) GetConnections(ctx context.Context, orgID, accessToken string) (map[string]interface{}, error) {
	log.Printf("Getting connections for organization %s", orgID)

	// Create the request
	req := MCPRequest{
		Method: "tools/call",
		Params: map[string]interface{}{
			"name": "get_connections",
			"arguments": map[string]interface{}{
				"orgId": orgID,
			},
		},
	}

	// Make the request with authentication
	return c.makeAuthenticatedRequest(ctx, req, accessToken)
}

// makeRequest makes a request to the MCP service
func (c *Client) makeRequest(ctx context.Context, req MCPRequest) (map[string]interface{}, error) {
	// Marshal the request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")

	// Make the request
	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	duration := time.Since(start)
	if err != nil {
		log.Printf("MCP service request failed after %v: %v", duration, err)
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log the response
	log.Printf("MCP service response - Status: %d, Duration: %v, Body: %s",
		resp.StatusCode, duration, string(body))

	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP service returned status code %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return result, nil
}

// makeAuthenticatedRequest makes an authenticated request to the MCP service
func (c *Client) makeAuthenticatedRequest(ctx context.Context, req MCPRequest, accessToken string) (map[string]interface{}, error) {
	// Marshal the request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	// Make the request
	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	duration := time.Since(start)
	if err != nil {
		log.Printf("MCP service request failed after %v: %v", duration, err)
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log the response
	log.Printf("MCP service response - Status: %d, Duration: %v, Body: %s",
		resp.StatusCode, duration, string(body))

	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP service returned status code %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return result, nil
}
