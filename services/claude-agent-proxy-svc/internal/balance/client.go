package balance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type BalanceData struct {
	Ticker        string      `json:"Ticker"`
	Amount       string      `json:"Amount"`
	WalletId     string      `json:"WalletId"`
	RemoteWalletId string    `json:"RemoteWalletId"`
	BlockId      string      `json:"BlockId"`
	TimestampSEC string      `json:"TimestampSEC"`
	RawMetadata  RawMetadata `json:"RawMetadata"`
}

type RawMetadata struct {
	Source      string      `json:"source"`
	Chain       string      `json:"chain"`
	RawResponse interface{} `json:"raw_response"`
}

type BalanceResponse struct {
	Success bool        `json:"success"`
	Data    *BalanceData `json:"data,omitempty"`
	Errors  []string    `json:"errors,omitempty"`
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: baseURL,
	}
}

func (c *Client) GetBalance(ctx context.Context, chain, address, tokenContract string) (*BalanceData, error) {
	payload := map[string]interface{}{
		"type":    "balance_query",
		"chain":   chain,
		"address": address,
	}

	if tokenContract != "" {
		payload["token_contract"] = tokenContract
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/pull", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var balanceResp BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balanceResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !balanceResp.Success {
		return nil, fmt.Errorf("balance service error: %v", balanceResp.Errors)
	}

	if balanceResp.Data == nil {
		return nil, fmt.Errorf("no data in response")
	}

	return balanceResp.Data, nil
}
