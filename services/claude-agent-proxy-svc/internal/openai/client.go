package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"
)

type Client struct {
	apiKey string
	model  string
	logger *slog.Logger
	client *http.Client
}

func NewClient(apiKey, model string, logger *slog.Logger) *Client {
	return &Client{
		apiKey: apiKey,
		model:  model,
		logger: logger,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ChatCompletion sends a single message to OpenAI without conversation history
func (c *Client) ChatCompletion(ctx context.Context, userMessage, correlationID string) (string, error) {
	messages := []Message{
		{
			Role:    "system",
			Content: "You are Wavie, a helpful AI assistant for Bitwave. You provide clear, concise, and helpful responses to user questions. Keep your responses professional but friendly.",
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}

	return c.sendChatRequest(ctx, messages, correlationID)
}

// Maximum number of conversation turns to include in history
const maxConversationTurns = 5

// Rough estimate of tokens per character for Claude API
const tokensPerChar = 0.25

// Maximum total tokens for Claude API
const maxTotalTokens = 200000

// Reserve tokens for system message, current message, and response
const reservedTokens = 10000

// ChatCompletionWithHistory sends a message to OpenAI with conversation history and optional knowledge context
func (c *Client) ChatCompletionWithHistory(ctx context.Context, userMessage string, history []Message, correlationID string, knowledgeContext string) (string, error) {
	// Create system message with optional knowledge context
	systemMessage := "You are Wavie, a helpful AI assistant for Bitwave. You provide clear, concise, and helpful responses to user questions. Keep your responses professional but friendly."
	
	// Add knowledge context if available
	if knowledgeContext != "" {
		systemMessage += "\n\nUse the following knowledge base to help answer the user's question. If the knowledge base doesn't contain relevant information, use your general knowledge but make it clear when you're doing so.\n\n" + knowledgeContext
	}

	// Estimate system message tokens
	systemTokens := int(math.Round(float64(len(systemMessage)) * tokensPerChar))
	
	// Estimate current user message tokens
	userMessageTokens := int(math.Round(float64(len(userMessage)) * tokensPerChar))
	
	// Calculate available tokens for history
	availableHistoryTokens := maxTotalTokens - systemTokens - userMessageTokens - reservedTokens
	
	// Start with system message
	messages := []Message{
		{
			Role:    "system",
			Content: systemMessage,
		},
	}

	// Add conversation history if available, but limit it
	if len(history) > 0 {
		// If history is too long, truncate it
		if len(history) > maxConversationTurns*2 { // *2 because each turn has user + assistant message
			c.logger.Info("Truncating conversation history due to turn limit", 
				"original_length", len(history), 
				"max_turns", maxConversationTurns,
				"new_length", maxConversationTurns*2)
			
			// Keep only the most recent messages
			history = history[len(history)-(maxConversationTurns*2):]
		}
		
		// Now check token limits
		var selectedHistory []Message
		historyTokens := 0
		
		// Start from most recent and work backwards
		for i := len(history) - 1; i >= 0; i-- {
			msgTokens := int(math.Round(float64(len(history[i].Content)) * tokensPerChar))
			
			if historyTokens + msgTokens > availableHistoryTokens {
				c.logger.Info("Truncating conversation history due to token limit", 
					"included_messages", len(selectedHistory), 
					"total_messages", len(history),
					"estimated_tokens", historyTokens)
				break
			}
			
			// Add this message to the selected history (at the beginning since we're going backwards)
			selectedHistory = append([]Message{{Role: history[i].Role, Content: history[i].Content}}, selectedHistory...)
			historyTokens += msgTokens
		}
		
		c.logger.Info("Adding conversation history", 
			"original_length", len(history), 
			"included_length", len(selectedHistory), 
			"estimated_tokens", historyTokens)
			
		messages = append(messages, selectedHistory...)
	}

	// Add the current user message
	messages = append(messages, Message{
		Role:    "user",
		Content: userMessage,
	})

	return c.sendChatRequest(ctx, messages, correlationID)
}

// sendChatRequest handles the actual API call to Claude API
func (c *Client) sendChatRequest(ctx context.Context, messages []Message, correlationID string) (string, error) {

	// Convert messages to Claude format
	systemMessage := ""
	userMessages := []string{}
	assistantMessages := []string{}

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			systemMessage = msg.Content
		case "user":
			userMessages = append(userMessages, msg.Content)
		case "assistant":
			assistantMessages = append(assistantMessages, msg.Content)
		}
	}

	// Build Claude API request
	request := ClaudeRequest{
		Model:       c.model,
		System:      systemMessage,
		Temperature: 0.7,
		MaxTokens:   1000,
	}

	// Add messages in conversation format
	for i := 0; i < len(userMessages); i++ {
		request.Messages = append(request.Messages, Message{Role: "user", Content: userMessages[i]})
		if i < len(assistantMessages) {
			request.Messages = append(request.Messages, Message{Role: "assistant", Content: assistantMessages[i]})
		}
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	c.logger.Info("Sending request to Claude API", "correlation_id", correlationID, "model", c.model)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return "", fmt.Errorf("Claude API error: %d - %s", resp.StatusCode, string(body))
		}
		return "", fmt.Errorf("Claude API error: %s", errorResp.Error.Message)
	}

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal Claude response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("no content in Claude response")
	}

	// Extract text from content blocks
	response := ""
	for _, block := range claudeResp.Content {
		if block.Type == "text" {
			response += block.Text
		}
	}

	c.logger.Info("Received response from Claude API",
		"correlation_id", correlationID,
		"tokens_used", claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		"response_length", len(response))

	return response, nil
}
