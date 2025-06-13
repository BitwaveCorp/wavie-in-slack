package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

// ChatCompletionWithHistory sends a message to OpenAI with conversation history and optional knowledge context
func (c *Client) ChatCompletionWithHistory(ctx context.Context, userMessage string, history []Message, correlationID string, knowledgeContext string) (string, error) {
	// Create system message with optional knowledge context
	systemMessage := "You are Wavie, a helpful AI assistant for Bitwave. You provide clear, concise, and helpful responses to user questions. Keep your responses professional but friendly."
	
	// Add knowledge context if available
	if knowledgeContext != "" {
		systemMessage += "\n\nUse the following knowledge base to help answer the user's question. If the knowledge base doesn't contain relevant information, use your general knowledge but make it clear when you're doing so.\n\n" + knowledgeContext
	}

	// Start with system message
	messages := []Message{
		{
			Role:    "system",
			Content: systemMessage,
		},
	}

	// Add conversation history if available
	if len(history) > 0 {
		c.logger.Info("Adding conversation history", "history_length", len(history))
		messages = append(messages, history...)
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
