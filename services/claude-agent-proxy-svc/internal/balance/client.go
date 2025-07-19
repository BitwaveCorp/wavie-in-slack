package balance

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
	// Build the URL according to the working curl command format
	url := fmt.Sprintf("%s/api/v1/chains/%s/addresses/%s/balance", c.baseURL, chain, address)
	if tokenContract != "" {
		url = fmt.Sprintf("%s?token=%s", url, tokenContract)
	}

	log.Printf("Sending balance request to: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		err = fmt.Errorf("failed to create request: %w", err)
		log.Printf("Error creating request: %v", err)
		return nil, err
	}

	req.Header.Set("Accept", "application/json")

	// Log request details
	headers, _ := json.Marshal(req.Header)
	log.Printf("Request headers: %s", string(headers))

	// Send the request
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to send request: %w", err)
		log.Printf("Error sending request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body for logging
	body, _ := io.ReadAll(resp.Body)
	// Create a new reader for the response body since we've already read it
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	log.Printf("Balance service response - Status: %d, Duration: %v, Headers: %v, Body: %s",
		resp.StatusCode,
		time.Since(start),
		resp.Header,
		string(body),
	)

	// Check status code
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
		log.Printf("Error response from balance service: %v", err)
		return nil, err
	}

	// Parse the response
	var balanceResp BalanceResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&balanceResp); err != nil {
		err = fmt.Errorf("failed to decode response: %w, response body: %s", err, string(body))
		log.Printf("Error decoding response: %v", err)
		return nil, err
	}

	if !balanceResp.Success {
		err = fmt.Errorf("balance service error: %v", balanceResp.Errors)
		log.Printf("Balance service returned error: %v", err)
		return nil, err
	}

	if balanceResp.Data == nil {
		err = fmt.Errorf("no data in response")
		log.Printf("No data in balance response")
		return nil, err
	}

	log.Printf("Successfully retrieved balance: %+v", balanceResp.Data)
	return balanceResp.Data, nil
}
