package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MCPQuery represents a query for MCP data
type MCPQuery struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	OrgID        string `json:"org_id"`
	FromSym      string `json:"from_sym"`
	ToFiat       string `json:"to_fiat"`
	Symbol       string `json:"symbol"`
}

// formatWalletsResponse formats the response for an organization wallets query
func formatWalletsResponse(query *MCPQuery, result map[string]interface{}) (string, map[string]interface{}) {
	// Extract the wallets data - first check for items array (actual structure)
	var resultData []interface{}
	
	// Try to get items array first (actual structure from API)
	if items, hasItems := result["items"].([]interface{}); hasItems {
		resultData = items
	} else if res, hasResult := result["result"].([]interface{}); hasResult {
		// Fallback to result array (previous expected structure)
		resultData = res
	} else {
		fmt.Println("Failed to parse wallets data", result)
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

func main() {
	// Sample data provided by the user (truncated for readability)
	jsonData := `{"items":[{"id":"b52e3d02-be0c-4c3c-b417-94f46180c638","name":"Binance","addresses":[],"orgId":"jtMvU6kEq7AF2DJIupb4","deleted":false,"disabled":false,"type":3,"groupId":"lvFd2FG5RrWL41PWzDgx","isBalanceMonitoringOnly":false,"isSyncEnabledSystem":true,"isSyncEnabledUser":true,"createdSEC":1708424164,"exchangeConnectionId":"ZHpdyGRKyOR3ITkXSns8"},{"id":"d8a4e7ff-389f-4c11-94e5-0188584b8148","name":"Bitgo","addresses":[],"orgId":"jtMvU6kEq7AF2DJIupb4","deleted":false,"disabled":false,"type":3,"isBalanceMonitoringOnly":false,"isSyncEnabledSystem":true,"isSyncEnabledUser":true,"createdSEC":1724380574,"exchangeConnectionId":"QzVGLw4ggTIjGe28nxjz"},{"id":"b03ba786-952b-4a33-b26f-a01fb1db526d","name":"CoinbasePrime","addresses":[],"orgId":"jtMvU6kEq7AF2DJIupb4","deleted":false,"disabled":false,"type":3,"groupId":"lvFd2FG5RrWL41PWzDgx","isBalanceMonitoringOnly":false,"isSyncEnabledSystem":true,"isSyncEnabledUser":true,"createdSEC":1708125739,"exchangeConnectionId":"wS93lbBT7zV7l65Ff4r0"},{"id":"af169379-d362-48f5-a527-60e7c8355802","name":"Kraken","addresses":[],"orgId":"jtMvU6kEq7AF2DJIupb4","deleted":false,"disabled":false,"type":3,"isBalanceMonitoringOnly":false,"isSyncEnabledSystem":true,"isSyncEnabledUser":true,"createdSEC":1706802611,"exchangeConnectionId":"wS6d8nPEra5vKjABIkiT"},{"id":"I4ZRHoW6FeyA4QSLVJWS","name":"Manual Wallet","description":"Manual Wallet for DemoSpecId","addresses":["Manual"],"orgId":"jtMvU6kEq7AF2DJIupb4","deleted":false,"disabled":false,"type":10,"isBalanceMonitoringOnly":false,"isSyncEnabledSystem":true,"isSyncEnabledUser":true,"createdSEC":1733332001,"subsidiaryId":"jtMvU6kEq7AF2DJIupb4"},{"id":"plMvw7jiU6PMfYoBW6gd","name":"Metamask","addresses":["0x4fF2Dd742cdAC4Ac79Ac6c4d445C40e3DbC30BBa"],"orgId":"jtMvU6kEq7AF2DJIupb4","deleted":false,"disabled":false,"type":1,"networkId":"eth","isBalanceMonitoringOnly":false,"isSyncEnabledSystem":true,"isSyncEnabledUser":true,"lastSuccessfulSyncSEC":1753079314,"lastSuccessfulBalanceCheckSEC":1723716123,"createdSEC":1711704928,"subsidiaryId":""}]}`

	// Parse the JSON data
	var result map[string]interface{}
	err := json.Unmarshal([]byte(jsonData), &result)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	// Create a sample query
	query := &MCPQuery{
		OrgID: "jtMvU6kEq7AF2DJIupb4",
	}

	// Format the wallets response
	formattedResponse, _ := formatWalletsResponse(query, result)

	// Print the formatted response
	fmt.Println("Formatted Wallets Response:")
	fmt.Println("==========================")
	fmt.Println(formattedResponse)
}
