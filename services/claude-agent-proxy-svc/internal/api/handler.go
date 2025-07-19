package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc/internal/balance"
	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc/internal/config"
	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc/internal/knowledge"
	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc/internal/openai"
)

type ConversationMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

type GPTRequest struct {
	Message             string                `json:"message"`
	UserID              string                `json:"user_id"`
	ChannelID           string                `json:"channel_id"`
	MessageTS           string                `json:"message_ts"`
	ThreadTS            string                `json:"thread_ts,omitempty"`
	ConversationHistory []ConversationMessage `json:"conversation_history,omitempty"`
	CorrelationID       string                `json:"correlation_id"`
	AgentID             string                `json:"agent_id,omitempty"`
}

type GPTResponse struct {
	Response      string `json:"response"`
	CorrelationID string `json:"correlation_id"`
	Error         string `json:"error,omitempty"`
}

// BalanceQuery represents a request to query a wallet balance
type BalanceQuery struct {
	Type          string `json:"type"`
	Chain         string `json:"chain"`
	Address       string `json:"address"`
	TokenContract string `json:"token_contract,omitempty"`
}

// BalanceResponse represents the response from the balance service
type BalanceResponse struct {
	Amount string `json:"amount"`
	Ticker string `json:"ticker"`
}

type ChatResponse struct {
	Response      string `json:"response"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// Handler handles API requests
type Handler struct {
	openaiClient *openai.Client
	knowledge    *knowledge.Retriever
	logger       *slog.Logger
	ragConfig    *config.RAGConfig
	balanceSvc   *balance.Client
}

// NewHandler creates a new API handler
func NewHandler(openaiClient *openai.Client, logger *slog.Logger, knowledgeRetriever *knowledge.Retriever, ragConfig *config.RAGConfig, balanceSvc *balance.Client) *Handler {
	return &Handler{
		openaiClient: openaiClient,
		knowledge:    knowledgeRetriever,
		logger:       logger,
		ragConfig:    ragConfig,
		balanceSvc:   balanceSvc,
	}
}

// safeTruncate safely truncates a string to show the first 'start' and last 'end' characters
// with '...' in between. If the string is shorter than start+end, returns the original string.
func safeTruncate(s string, start, end int) string {
	if len(s) <= start+end {
		return s
	}
	// Ensure we don't try to slice beyond the string length
	startChars := start
	if startChars > len(s) {
		startChars = len(s)
	}
	endChars := end
	if endChars > len(s) {
		endChars = len(s)
	}
	return s[:startChars] + "..." + s[len(s)-endChars:]
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.handleHealthCheck)
	mux.HandleFunc("POST /api/chat", h.handleChatCompletion)
}

func (h *Handler) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{"status": "ok"}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// detectBalanceQuery checks if the message is a balance query and extracts parameters
// using Claude's intelligence to determine the query type and extract parameters
func (h *Handler) detectBalanceQuery(message string) (*BalanceQuery, bool) {
	h.logger.Debug("Asking Claude to detect if message is a balance query", "message", message)

	// Ask Claude to detect if this is a balance query and extract parameters
	prompt := `You are Wavie, a digital assets accounting expert agent, whose primary expertise is in having full knowledge of the bitwave platform based on its user guide that's in your knowledge base. You are here to answer user questions, solve user problems and overall help them be successful using the bitwave platform. You have 2 main features:

1. Answer general queries: 100% based on the knowledge base provided to you which is a full bitwave platform user guide / knowledge base
2. Answer balance queries: providing cryptocurrency wallet balance information upon request.

You will receive a message and must determine if it is one of these two types:
1. A balance query - A request to check a cryptocurrency wallet balance
2. A general query - Any other type of message related to the bitwave platform

For balance queries, you MUST respond with a JSON object containing these fields:
- type: Always "balance_query"
- chain: The blockchain network (e.g., "ethereum", "bitcoin", "polygon")
- address: The wallet address
- token_contract: (Optional) The token contract address if it's a token balance query

For general queries, respond with an empty JSON object: {}

Examples of balance queries:
- "What's my Ethereum balance? 0x123..."
  {"type":"balance_query","chain":"ethereum","address":"0x123..."}
- "Check my Bitcoin balance for bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
  {"type":"balance_query","chain":"bitcoin","address":"bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"}
- "Show my USDT balance on Ethereum 0x456..."
  {"type":"balance_query","chain":"ethereum","address":"0x456...","token_contract":"0xdac17f958d2ee523a2206206994597c13d831ec7"}

Examples of general queries:
- "What's the weather like today?"
  {}
- "Explain how blockchain works"
  {}

Now analyze this message (respond with only the JSON object, no other text or formatting):
` + message

	// Create a background context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	h.logger.Debug("Sending message to Claude for balance query detection")

	// Call Claude with the prompt
	resp, err := h.openaiClient.CreateChatCompletion(ctx, []openai.ChatMessage{
		{
			Role:    "system",
			Content: "You are a classifier that determines if a message is a cryptocurrency wallet balance query. Respond with only a JSON object, no other text or formatting.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}, 0.0) // Very low temperature for deterministic output

	if err != nil {
		h.logger.Error("Failed to call Claude for balance query detection", "error", err)
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
	var query BalanceQuery
	if err := json.Unmarshal([]byte(jsonStr), &query); err != nil {
		h.logger.Error("Failed to parse Claude's response as JSON", "error", err, "json", jsonStr)
		return nil, false
	}

	// If type is not balance_query, this is not a balance query
	if query.Type != "balance_query" {
		h.logger.Debug("Claude determined this is not a balance query")
		return nil, false
	}

	// Clean up the extracted values
	query.Chain = strings.TrimSpace(strings.ToLower(query.Chain))
	query.Address = strings.TrimSpace(query.Address)

	// Remove any quotes or special characters that might have been included
	query.Chain = strings.Trim(query.Chain, `"'`)
	query.Address = strings.Trim(query.Address, `"' `)

	// Log the detected query
	h.logger.Info("Claude detected a balance query", 
		"chain", query.Chain, 
		"address", query.Address,
		"has_token_contract", query.TokenContract != "")

	return &query, true
}

// handleBalanceQuery processes a wallet balance query
func (h *Handler) handleBalanceQuery(w http.ResponseWriter, r *http.Request, query *BalanceQuery, correlationID string) {
	h.logger.Info("Processing balance query", 
		"chain", query.Chain, 
		"address", query.Address,
		"token_contract", query.TokenContract,
		"correlation_id", correlationID)

	// Set content type for the response
	w.Header().Set("Content-Type", "application/json")

	// Get the balance from the balance service
	balance, err := h.balanceSvc.GetBalance(r.Context(), query.Chain, query.Address, query.TokenContract)
	if err != nil {
		h.logger.Error("Failed to get balance", 
			"error", err, 
			"correlation_id", correlationID,
			"chain", query.Chain,
		)
		
		// Format error response
		errResponse := ChatResponse{
			Response:      "❌ I couldn't retrieve the balance at this time. Please try again later.",
			CorrelationID: correlationID,
		}
		
		if err := json.NewEncoder(w).Encode(errResponse); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	// Format the address for display
	displayAddress := query.Address
	if len(displayAddress) > 10 {
		// Safely truncate the address
		startLen := 6
		endLen := 4
		
		// Make sure we don't try to take more characters than available
		if len(displayAddress) < startLen + endLen {
			// If address is too short, just use it as is
			// This should never happen due to the len > 10 check above, but adding as a safeguard
		} else {
			displayAddress = displayAddress[:startLen] + "..." + displayAddress[len(displayAddress)-endLen:]
		}
	}

	// Format the response
	var response string
	var responseData map[string]interface{}

	if query.TokenContract != "" {
		// Token balance response
		tokenName := balance.Ticker
		if tokenName == "" {
			tokenName = "tokens"
		}

		response = fmt.Sprintf("🔹 *%s Token Balance*\n\n"+
			"*Wallet:* `%s`\n"+
			"*Chain:* %s\n"+
			"*Token Contract:* `%s`\n"+
			"*Balance:* %s %s\n"+
			"*Last Updated:* %s",
			strings.ToUpper(query.Chain),
			displayAddress,
			strings.Title(query.Chain),
			safeTruncate(query.TokenContract, 6, 4),
			balance.Amount,
			tokenName,
			time.Now().Format("January 2, 2006 15:04:05 MST"),
		)

		responseData = map[string]interface{}{
			"type":            "token_balance",
			"chain":           query.Chain,
			"address":         query.Address,
			"token_contract":  query.TokenContract,
			"balance":         balance.Amount,
			"ticker":          balance.Ticker,
			"formatted":       response,
			"last_updated":    time.Now().Format(time.RFC3339),
		}
	} else {
		// Native token balance response
		response = fmt.Sprintf("🔹 *%s Balance*\n\n"+
			"*Wallet:* `%s`\n"+
			"*Balance:* %s %s\n"+
			"*Last Updated:* %s",
			strings.ToUpper(query.Chain),
			displayAddress,
			balance.Amount,
			balance.Ticker,
			time.Now().Format("January 2, 2006 15:04:05 MST"),
		)

		responseData = map[string]interface{}{
			"type":          "native_balance",
			"chain":         query.Chain,
			"address":       query.Address,
			"balance":       balance.Amount,
			"ticker":        balance.Ticker,
			"formatted":     response,
			"last_updated":  time.Now().Format(time.RFC3339),
		}
	}

	// Log the successful response
	h.logger.Info("Successfully retrieved balance",
		"chain", query.Chain,
		"address", displayAddress,
		"balance", balance.Amount,
		"ticker", balance.Ticker,
	)

	// Create the response
	resp := map[string]interface{}{
		"response":       response,
		"correlation_id": correlationID,
		"data":           responseData,
	}

	// Send the response
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("Failed to encode balance response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	// Parse the request body first
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Log the raw request body for debugging
	h.logger.Debug("Raw request body", "body", string(body))

	// Parse the JSON request
	var req GPTRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.logger.Error("Failed to parse request body", "error", err, "body", string(body))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set correlation ID if not provided
	if req.CorrelationID == "" {
		req.CorrelationID = fmt.Sprintf("chat_%d", time.Now().UnixNano())
	}

	h.logger.Info("Processing chat completion request",
		"correlation_id", req.CorrelationID,
		"user_id", req.UserID,
		"channel_id", req.ChannelID,
		"message_ts", req.MessageTS,
		"message_length", len(req.Message),
	)

	if req.Message == "" {
		h.logger.Error("Empty message in request", "correlation_id", req.CorrelationID)
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Check if this is a balance query
	if balanceQuery, isBalance := h.detectBalanceQuery(req.Message); isBalance && balanceQuery != nil {
		h.logger.Info("Detected balance query", 
			"correlation_id", req.CorrelationID,
			"chain", balanceQuery.Chain,
			"address", safeTruncate(balanceQuery.Address, 6, 4),
		)
		h.handleBalanceQuery(w, r, balanceQuery, req.CorrelationID)
		return
	}

	h.logger.Debug("Not a balance query, processing as regular chat completion")

	h.logger.Info("Processing as regular chat completion", "correlation_id", req.CorrelationID)

	h.logger.Info("Processing chat completion request",
		"correlation_id", req.CorrelationID,
		"user_id", req.UserID,
		"channel_id", req.ChannelID,
		"thread_ts", req.ThreadTS,
		"message", req.Message,
		"has_history", len(req.ConversationHistory) > 0)

	// Check if this is a feedback message
	if isFeedbackMessage := h.handleFeedbackMessage(w, req); isFeedbackMessage {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	// Convert ConversationMessage to openai.Message
	openaiMessages := convertToOpenAIMessages(req.ConversationHistory)

	// Get agent ID, default to "wavie-bot" if not specified
	agentID := req.AgentID
	if agentID == "" {
		agentID = "wavie-bot"
	}

	// Add knowledge context if available
	var knowledgeContext string

	// First try to get context from RAG service if enabled
	var ragContext string
	if h.ragConfig != nil && h.ragConfig.Enabled && h.ragConfig.URL != "" {
		h.logger.Info("Retrieving context from RAG service",
			"agent_id", agentID,
			"rag_url", h.ragConfig.URL)

		// Create request payload
		reqBody, err := json.Marshal(map[string]string{
			"question": req.Message,
		})
		if err != nil {
			h.logger.Error("Failed to marshal RAG ask request", "error", err)
		} else {
			// Send to RAG service
			start := time.Now()
			resp, err := http.Post(
				h.ragConfig.URL+"/api/ask",
				"application/json",
				bytes.NewBuffer(reqBody),
			)
			retrievalTime := time.Since(start)

			if err != nil {
				h.logger.Error("Failed to send query to RAG service", "error", err)
			} else {
				defer resp.Body.Close()

				// Check response
				if resp.StatusCode != http.StatusOK {
					respBody, _ := io.ReadAll(resp.Body)
					respBodyStr := string(respBody)
					
					// Special handling for 501 Not Implemented error from Vertex AI Vector Search
					if resp.StatusCode == http.StatusInternalServerError && 
					   strings.Contains(respBodyStr, "501") && 
					   strings.Contains(respBodyStr, "UNIMPLEMENTED") {
						h.logger.Warn("RAG service vector search not implemented or enabled", 
							"status", resp.Status,
							"response", respBodyStr,
							"fallback", "Using traditional knowledge retrieval")
					} else {
						h.logger.Error("RAG service returned error",
							"status", resp.Status,
							"response", respBodyStr)
					}
				} else {
					// Parse response
					var ragResp struct {
						Answer       string   `json:"answer"`
						SourceChunks []string `json:"source_chunks"`
					}

					if err := json.NewDecoder(resp.Body).Decode(&ragResp); err != nil {
						h.logger.Error("Failed to decode RAG response", "error", err)
					} else if len(ragResp.SourceChunks) > 0 {
						// Build context from source chunks
						var builder strings.Builder
						builder.WriteString("# " + ragResp.Answer + "\n\n")

						for i, chunk := range ragResp.SourceChunks {
							builder.WriteString(fmt.Sprintf("## Document %d\n\n%s\n\n", i+1, chunk))
						}

						ragContext = builder.String()
						h.logger.Info("Successfully retrieved context from RAG service",
							"chunk_count", len(ragResp.SourceChunks),
							"context_length", len(ragContext),
							"retrieval_time_ms", retrievalTime.Milliseconds())
					} else {
						h.logger.Info("No relevant chunks found in RAG service")
					}
				}
			}
		}
	}

	// Then try traditional knowledge retrieval if RAG didn't provide context
	if ragContext == "" && h.knowledge != nil {
		h.logger.Info("Retrieving knowledge context for agent",
			"agent_id", agentID,
			"storage_type", h.knowledge.GetStorageBackendType())

		start := time.Now()
		context, err := h.knowledge.GetKnowledgeContext(agentID)
		retrievalTime := time.Since(start)

		if err != nil {
			h.logger.Warn("Failed to get knowledge context",
				"error", err,
				"agent_id", agentID,
				"storage_type", h.knowledge.GetStorageBackendType())
		} else if context != "" {
			knowledgeContext = context
			h.logger.Info("Successfully retrieved knowledge context",
				"agent_id", agentID,
				"storage_type", h.knowledge.GetStorageBackendType(),
				"context_length", len(knowledgeContext),
				"retrieval_time_ms", retrievalTime.Milliseconds(),
				"feeding_to_claude", true)
		} else {
			h.logger.Info("No knowledge context available for agent",
				"agent_id", agentID,
				"storage_type", h.knowledge.GetStorageBackendType())
		}
	}

	// Use RAG context if available, otherwise use knowledge context
	if ragContext != "" {
		knowledgeContext = ragContext
	}

	// Use conversation history if available
	response, err := h.openaiClient.ChatCompletionWithHistory(ctx, req.Message, openaiMessages, req.CorrelationID, knowledgeContext)
	if err != nil {
		h.logger.Error("Failed to get chat completion", "error", err, "correlation_id", req.CorrelationID)

		gptResp := GPTResponse{
			CorrelationID: req.CorrelationID,
			Error:         err.Error(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(gptResp)
		return
	}

	gptResp := GPTResponse{
		Response:      response,
		CorrelationID: req.CorrelationID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(gptResp)

	h.logger.Info("Successfully processed chat completion", "correlation_id", req.CorrelationID)
}

// SetupRoutes is deprecated, use RegisterRoutes instead
func (h *Handler) SetupRoutes(router *http.ServeMux) {
	// This function is not used, the actual route registration is done in RegisterRoutes
}

// Helper function to convert ConversationMessage to openai.Message
func convertToOpenAIMessages(messages []ConversationMessage) []openai.Message {
	openaiMessages := make([]openai.Message, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return openaiMessages
}

// handleFeedbackMessage checks if a message is a feedback message and handles it appropriately
// Returns true if the message was handled as feedback, false otherwise
func (h *Handler) handleFeedbackMessage(w http.ResponseWriter, req GPTRequest) bool {
	// Check if this is a reaction feedback message
	if req.Message == "FEEDBACK_REACTION:closed_book" {
		h.logger.Info("Processing reaction feedback message",
			"correlation_id", req.CorrelationID,
			"user_id", req.UserID,
			"channel_id", req.ChannelID,
			"thread_ts", req.ThreadTS)

		// Create a standard acknowledgment response for feedback
		response := "FEEDBACK Noted :closed_book: Thank you for your feedback. We'll work to improve our responses."

		// Return the response
		gptResp := GPTResponse{
			Response:      response,
			CorrelationID: req.CorrelationID,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(gptResp)

		h.logger.Info("Successfully processed reaction feedback message", "correlation_id", req.CorrelationID)
		return true
	}

	// Check for text feedback messages with FEEDBACK_TEXT: prefix
	if strings.HasPrefix(req.Message, "FEEDBACK_TEXT:") {
		feedbackText := strings.TrimPrefix(req.Message, "FEEDBACK_TEXT:")
		h.logger.Info("Processing text feedback message",
			"correlation_id", req.CorrelationID,
			"user_id", req.UserID,
			"channel_id", req.ChannelID,
			"thread_ts", req.ThreadTS,
			"feedback_text", feedbackText)

		// Create a standard acknowledgment response for text feedback
		response := "FEEDBACK Noted :closed_book: Thank you for your detailed feedback. We'll work to address your concerns."

		// Return the response
		gptResp := GPTResponse{
			Response:      response,
			CorrelationID: req.CorrelationID,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(gptResp)

		h.logger.Info("Successfully processed text feedback message", "correlation_id", req.CorrelationID)
		return true
	}

	// Also check for legacy text feedback messages that start with ***
	if len(req.Message) > 3 && req.Message[:3] == "***" {
		h.logger.Info("Processing legacy detailed feedback message",
			"correlation_id", req.CorrelationID,
			"user_id", req.UserID,
			"channel_id", req.ChannelID,
			"thread_ts", req.ThreadTS)

		// Create a standard acknowledgment response for detailed feedback
		response := "FEEDBACK Noted :closed_book: Thank you for your detailed feedback. We'll work to address your concerns."

		// Return the response
		gptResp := GPTResponse{
			Response:      response,
			CorrelationID: req.CorrelationID,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(gptResp)

		h.logger.Info("Successfully processed legacy detailed feedback message", "correlation_id", req.CorrelationID)
		return true
	}

	return false
}
