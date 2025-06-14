package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/slack-events-listener-svc/internal/conversation"
	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/slack-events-listener-svc/internal/idgen"
	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/slack-events-listener-svc/internal/slack"
	"github.com/google/uuid"
)

type Handler struct {
	slackClient           *slack.Client
	signingSecret         string
	claudeProxyServiceURL string
	broadcastServiceURL   string
	logger                *slog.Logger
	processedEvents       map[string]bool
	eventsMutex           sync.RWMutex
	conversationStore     *conversation.Store
	agentID               string // The agent ID for this bot instance
}

func NewHandler(slackClient *slack.Client, signingSecret, claudeProxyServiceURL, broadcastServiceURL, agentID string, logger *slog.Logger) *Handler {
	// Create conversation store with 10 message limit, 15 minutes max age, and 500 character limit per message
	conversationStore := conversation.NewStoreWithMessageLimit(10, 15*time.Minute, 500)

	return &Handler{
		slackClient:           slackClient,
		signingSecret:         signingSecret,
		claudeProxyServiceURL: claudeProxyServiceURL,
		broadcastServiceURL:   broadcastServiceURL,
		logger:                logger,
		processedEvents:       make(map[string]bool),
		conversationStore:     conversationStore,
		agentID:               agentID,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.handleHealthCheck)
	mux.HandleFunc("POST /slack/events", h.ProcessEvent)
}

// Helper function to get the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (h *Handler) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{"status": "ok"}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) verifySlackSignature(r *http.Request) error {
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")

	if timestamp == "" || signature == "" {
		return fmt.Errorf("missing timestamp or signature")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse timestamp: %w", err)
	}

	if time.Now().Unix()-ts > 300 {
		return fmt.Errorf("timestamp is too old")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	// Restore the request body so it can be read again
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(h.signingSecret))
	mac.Write([]byte(baseString))
	expectedSignature := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedSignature), []byte(signature)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

func (h *Handler) isEventProcessed(eventID string) bool {
	h.eventsMutex.RLock()
	defer h.eventsMutex.RUnlock()
	return h.processedEvents[eventID]
}

func (h *Handler) markEventProcessed(eventID string) {
	h.eventsMutex.Lock()
	defer h.eventsMutex.Unlock()
	h.processedEvents[eventID] = true
}

func (h *Handler) ProcessEvent(w http.ResponseWriter, r *http.Request) {
	// Log request headers for debugging
	h.logger.Info("Received Slack event request",
		"method", r.Method,
		"path", r.URL.Path,
		"content_type", r.Header.Get("Content-Type"),
		"content_length", r.Header.Get("Content-Length"))

	// Read request body first so we can use it for both signature verification and event processing
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Log the raw request body for debugging
	h.logger.Info("Received Slack event body", "body_length", len(body))
	if len(body) < 1000 { // Only log full body if it's not too large
		h.logger.Debug("Received Slack event body content", "body", string(body))
	}

	// Check if body is empty
	if len(body) == 0 {
		h.logger.Error("Empty request body")
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}

	// Create a new reader with the body content and replace the request body
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Verify Slack signature
	if err := h.verifySlackSignature(r); err != nil {
		h.logger.Error("Failed to verify Slack signature", "error", err)
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Reset the request body again for parsing
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Parse event
	var eventReq slack.EventRequest
	if err := json.Unmarshal(body, &eventReq); err != nil {
		h.logger.Error("Failed to parse event request", "error", err, "body_preview", string(body[:min(len(body), 200)]))
		http.Error(w, "Failed to parse event request", http.StatusBadRequest)
		return
	}

	// Log the parsed event for debugging
	h.logger.Debug("Parsed Slack event", "type", eventReq.Type, "event_id", eventReq.EventID)

	// Handle URL verification challenge
	if eventReq.Type == "url_verification" {
		h.logger.Info("Received URL verification challenge", "challenge", eventReq.Challenge)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"challenge": eventReq.Challenge})
		return
	}

	// Deduplicate events
	if h.isEventProcessed(eventReq.EventID) {
		h.logger.Info("Duplicate event received, ignoring", "event_id", eventReq.EventID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Process event asynchronously
	go func() {
		switch eventReq.Event.Type {
		case "app_mention":
			h.handleAppMention(eventReq)
		case "reaction_added":
			h.handleReactionAdded(eventReq)
		case "message":
			// Log all message events for debugging
			prefix := ""
			if len(eventReq.Event.Text) > 10 {
				prefix = eventReq.Event.Text[:10] + "..."
			} else {
				prefix = eventReq.Event.Text
			}
			
			h.logger.Info("Received message event",
				"user", eventReq.Event.User,
				"channel", eventReq.Event.Channel,
				"has_thread_ts", eventReq.Event.ThreadTS != "",
				"text_prefix", prefix)
			
			// Note: We no longer process *** text feedback in messages
			// Feedback is now handled via reactions or @wavie mentions with closed_book
		}
		h.markEventProcessed(eventReq.EventID)
	}()

	// Respond immediately to Slack
	w.WriteHeader(http.StatusOK)
}

// handleReactionAdded processes reaction events for feedback
func (h *Handler) handleReactionAdded(eventReq slack.EventRequest) {
	// Only process closed_book reactions for negative feedback
	if eventReq.Event.Reaction != "closed_book" {
		return
	}

	// Get the message that was reacted to
	messageTS := eventReq.Event.Item.TS
	channel := eventReq.Event.Item.Channel

	// Create a correlation ID for this feedback
	correlationID := "fb_" + uuid.New().String()

	// Set feedback type to negative since we only handle closed_book reactions
	feedbackType := "negative"

	h.logger.Info("Processing reaction feedback",
		"feedback_type", feedbackType,
		"user", eventReq.Event.User,
		"channel", channel,
		"correlation_id", correlationID)

	// Add a log message similar to normal message processing for consistency
	h.logger.Info("Processing wavie message",
		"channel", channel,
		"user", eventReq.Event.User,
		"is_thread", true,
		"thread_id", messageTS,
		"correlation_id", correlationID)

	// Send feedback through Claude proxy as a special message
	// This will follow the normal message flow but with a special format
	message := "FEEDBACK_REACTION:closed_book"

	// Get conversation history for this thread
	threadID := messageTS // Use the message timestamp as thread ID
	conversationHistory := h.conversationStore.GetMessages(threadID)

	// Convert conversation.Message to slack.ConversationMessage
	slackConversationHistory := convertToSlackMessages(conversationHistory)

	// Create a Claude request with the feedback message
	claudeReq := slack.ClaudeRequest{
		Message:             message,
		UserID:              eventReq.Event.User,
		ChannelID:           channel,
		MessageTS:           messageTS, // Using the messageTS we already extracted
		ThreadTS:            threadID,
		ConversationHistory: slackConversationHistory,
		CorrelationID:       correlationID,
		AgentID:             h.agentID,
	}

	// Call Claude service with the feedback message
	claudeResp, err := h.callClaudeService(claudeReq)
	if err != nil {
		h.logger.Error("Failed to call Claude service for feedback", "error", err, "correlation_id", correlationID)
		return
	}

	// Post the Claude response back to the thread
	err = h.slackClient.PostMessage(context.Background(), channel, claudeResp.Response, threadID)
	if err != nil {
		h.logger.Error("Failed to post feedback response to Slack", "error", err, "correlation_id", correlationID)
		return
	}

	// Send to broadcast service as well for consistency
	broadcastReq := slack.BroadcastRequest{
		UserID:        eventReq.Event.User,
		ChannelID:     channel,
		ThreadID:      threadID,
		Question:      message,
		Response:      claudeResp.Response,
		Timestamp:     time.Now(),
		CorrelationID: correlationID,
	}

	go h.callBroadcastService(broadcastReq)

	h.logger.Info("Processed feedback through Claude proxy",
		"feedback_type", feedbackType,
		"user", eventReq.Event.User,
		"channel", channel,
		"correlation_id", correlationID)
}

// handleTextFeedback processes text feedback from thread replies
func (h *Handler) handleTextFeedback(eventReq slack.EventRequest) {
	// Extract feedback text (remove the *** prefix)
	feedbackText := strings.TrimPrefix(eventReq.Event.Text, "***")
	feedbackText = strings.TrimSpace(feedbackText)

	// If no actual feedback text, ignore
	if feedbackText == "" {
		return
	}

	// Create a correlation ID for this feedback
	correlationID := "fb_" + uuid.New().String()

	// Get the thread ID (parent message)
	threadID := eventReq.Event.ThreadTS
	channel := eventReq.Event.Channel

	// Log processing of feedback message
	h.logger.Info("Processing wavie message",
		"channel", channel,
		"user", eventReq.Event.User,
		"is_thread", true,
		"thread_id", threadID,
		"correlation_id", correlationID)

	// Get conversation history for context
	conversationHistory := h.conversationStore.GetMessages(threadID)

	// Create a special message format for text feedback
	message := fmt.Sprintf("FEEDBACK_TEXT:%s", feedbackText)

	// Convert conversation.Message to slack.ConversationMessage
	slackConversationHistory := convertToSlackMessages(conversationHistory)

	// Create a Claude request with the feedback message
	claudeReq := slack.ClaudeRequest{
		Message:             message,
		UserID:              eventReq.Event.User,
		ChannelID:           channel,
		MessageTS:           eventReq.Event.TS,
		ThreadTS:            threadID,
		ConversationHistory: slackConversationHistory,
		CorrelationID:       correlationID,
		AgentID:             h.agentID,
	}

	// Call Claude service with the feedback message
	claudeResp, err := h.callClaudeService(claudeReq)
	if err != nil {
		h.logger.Error("Failed to send text feedback to Claude service", "error", err)
		return
	}

	h.logger.Info("Processed text feedback",
		"user", eventReq.Event.User,
		"channel", eventReq.Event.Channel,
		"correlation_id", correlationID,
		"claude_response", claudeResp)
}

// sendFeedbackToBroadcast sends feedback to the broadcast service
func (h *Handler) sendFeedbackToBroadcast(feedback slack.FeedbackRequest) {
	// Marshal feedback to JSON
	feedbackJSON, err := json.Marshal(feedback)
	if err != nil {
		h.logger.Error("Failed to marshal feedback", "error", err)
		return
	}

	// Send to broadcast service
	resp, err := http.Post(h.broadcastServiceURL+"/api/feedback", "application/json", bytes.NewReader(feedbackJSON))
	if err != nil {
		h.logger.Error("Failed to send feedback to broadcast service", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.logger.Error("Broadcast service returned non-OK status", "status", resp.Status)
		return
	}

	h.logger.Info("Successfully sent feedback to broadcast service", "correlation_id", feedback.CorrelationID)
}

func (h *Handler) handleAppMention(eventReq slack.EventRequest) {
	correlationID, err := idgen.GenerateId("wv", 16)
	if err != nil {
		h.logger.Error("Failed to generate correlation ID", "error", err)
		return
	}

	// Determine if this is a thread reply or a new message
	isThreadReply := eventReq.Event.ThreadTS != ""
	threadID := eventReq.Event.ThreadTS
	if threadID == "" {
		threadID = eventReq.Event.TS // Use message timestamp as thread ID for new messages
	}

	// Get the original message text before cleaning
	originalMessage := eventReq.Event.Text

	// Check if this is a feedback message containing "closed_book"
	if strings.Contains(strings.ToLower(originalMessage), "closed_book") {
		h.logger.Info("Processing feedback from @mention with closed_book",
			"correlation_id", correlationID,
			"user", eventReq.Event.User,
			"channel", eventReq.Event.Channel,
			"is_thread", isThreadReply,
			"thread_id", threadID)
		
		// Extract feedback text (everything after removing mentions)
		feedbackText := strings.ReplaceAll(originalMessage, "<@", "")
		feedbackText = strings.ReplaceAll(feedbackText, ">", "")
		feedbackText = strings.ReplaceAll(feedbackText, "@wavie", "")
		feedbackText = strings.TrimSpace(feedbackText)
		
		// Create a feedback request similar to handleReactionAdded
		// Get conversation history for context
		conversationHistory := h.conversationStore.GetMessages(threadID)
		
		// Create a special message format for text feedback
		message := fmt.Sprintf("FEEDBACK_TEXT:%s", feedbackText)
		
		// Convert conversation.Message to slack.ConversationMessage
		slackConversationHistory := convertToSlackMessages(conversationHistory)
		
		// Create a Claude request with the feedback message
		claudeReq := slack.ClaudeRequest{
			Message:             message,
			UserID:              eventReq.Event.User,
			ChannelID:           eventReq.Event.Channel,
			MessageTS:           eventReq.Event.TS,
			ThreadTS:            threadID,
			ConversationHistory: slackConversationHistory,
			CorrelationID:       correlationID,
			AgentID:             h.agentID,
		}
		
		// Call Claude service with the feedback message
		claudeResp, err := h.callClaudeService(claudeReq)
		if err != nil {
			h.logger.Error("Failed to call Claude service for feedback", "error", err, "correlation_id", correlationID)
			return
		}
		
		// Post the Claude response back to the thread
		err = h.slackClient.PostMessage(context.Background(), eventReq.Event.Channel, claudeResp.Response, threadID)
		if err != nil {
			h.logger.Error("Failed to post feedback response to Slack", "error", err, "correlation_id", correlationID)
			return
		}
		
		// Send to broadcast service as well for consistency
		broadcastReq := slack.BroadcastRequest{
			UserID:        eventReq.Event.User,
			ChannelID:     eventReq.Event.Channel,
			ThreadID:      threadID,
			Question:      message,
			Response:      claudeResp.Response,
			Timestamp:     time.Now(),
			CorrelationID: correlationID,
		}
		
		go h.callBroadcastService(broadcastReq)
		
		h.logger.Info("Processed feedback through Claude proxy",
			"feedback_type", "text_with_closed_book",
			"user", eventReq.Event.User,
			"channel", eventReq.Event.Channel,
			"correlation_id", correlationID)
		return
	}

	h.logger.Info("Processing regular wavie message",
		"correlation_id", correlationID,
		"user", eventReq.Event.User,
		"channel", eventReq.Event.Channel,
		"is_thread", isThreadReply,
		"thread_id", threadID)

	// Clean the message text
	message := strings.ReplaceAll(eventReq.Event.Text, "<@", "")
	message = strings.ReplaceAll(message, ">", "")
	message = strings.ReplaceAll(message, "@wavie", "")
	message = strings.TrimSpace(message)

	// Add user message to conversation context
	h.conversationStore.AddMessage(threadID, "user", message)

	// Get conversation history for this thread
	conversationHistory := h.conversationStore.GetMessages(threadID)

	// Convert conversation.Message to slack.ConversationMessage
	slackConversationHistory := convertToSlackMessages(conversationHistory)

	// Use the agent ID associated with this bot instance
	h.logger.Info("Using configured agent for request", "agent_id", h.agentID, "correlation_id", correlationID)

	claudeReq := slack.ClaudeRequest{
		Message:             message,
		UserID:              eventReq.Event.User,
		ChannelID:           eventReq.Event.Channel,
		MessageTS:           eventReq.Event.TS,
		ThreadTS:            threadID,
		ConversationHistory: slackConversationHistory,
		CorrelationID:       correlationID,
		AgentID:             h.agentID,
	}

	claudeResp, err := h.callClaudeService(claudeReq)
	if err != nil {
		h.logger.Error("Failed to call GPT service", "error", err, "correlation_id", correlationID)
		h.slackClient.PostMessage(context.Background(), eventReq.Event.Channel, "Sorry, I'm having trouble processing your request right now.", threadID)
		return
	}

	if claudeResp.Error != "" {
		h.logger.Error("Claude service returned error", "error", claudeResp.Error, "correlation_id", correlationID)
		h.slackClient.PostMessage(context.Background(), eventReq.Event.Channel, "Sorry, I encountered an error processing your request.", threadID)
		return
	}

	// Add bot response to conversation context
	h.conversationStore.AddMessage(threadID, "assistant", claudeResp.Response)

	// For new conversations (not in a thread), append a hint to continue conversation in thread for new messages
	if eventReq.Event.ThreadTS == "" {
		claudeResp.Response += "\n\n_Reply in this thread to continue our conversation. React with :closed_book: if the response needs improvement, or within the thread mention @wavie with :closed_book: to leave detailed feedback._"
	}

	// Always reply in the thread if there is one
	err = h.slackClient.PostMessage(context.Background(), eventReq.Event.Channel, claudeResp.Response, threadID)
	if err != nil {
		h.logger.Error("Failed to post response to Slack", "error", err, "correlation_id", correlationID)
		return
	}

	broadcastReq := slack.BroadcastRequest{
		UserID:        eventReq.Event.User,
		ChannelID:     eventReq.Event.Channel,
		ThreadID:      threadID,
		Question:      message,
		Response:      claudeResp.Response,
		Timestamp:     time.Now(),
		CorrelationID: correlationID,
	}

	go h.callBroadcastService(broadcastReq)
}

func (h *Handler) callClaudeService(req slack.ClaudeRequest) (slack.ClaudeResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	url := h.claudeProxyServiceURL + "/api/chat"

	reqBody, err := json.Marshal(req)
	if err != nil {
		return slack.ClaudeResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return slack.ClaudeResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Create a custom transport with increased timeouts
	transport := &http.Transport{
		TLSHandshakeTimeout: 30 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}

	client := &http.Client{
		Timeout:   90 * time.Second,
		Transport: transport,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return slack.ClaudeResponse{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return slack.ClaudeResponse{}, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var claudeResp slack.ClaudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return slack.ClaudeResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return claudeResp, nil
}

func (h *Handler) callBroadcastService(req slack.BroadcastRequest) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		h.logger.Error("Failed to marshal broadcast request", "error", err, "correlation_id", req.CorrelationID)
		return
	}

	httpReq, err := http.NewRequest("POST", h.broadcastServiceURL+"/api/broadcast", bytes.NewBuffer(jsonData))
	if err != nil {
		h.logger.Error("Failed to create broadcast request", "error", err, "correlation_id", req.CorrelationID)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		h.logger.Error("Failed to call broadcast service", "error", err, "correlation_id", req.CorrelationID)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		h.logger.Error("Broadcast service error", "status_code", resp.StatusCode, "body", string(body), "correlation_id", req.CorrelationID)
	}
}

// Helper function to convert conversation.Message to slack.ConversationMessage
func convertToSlackMessages(messages []conversation.Message) []slack.ConversationMessage {
	slackMessages := make([]slack.ConversationMessage, len(messages))
	for i, msg := range messages {
		slackMessages[i] = slack.ConversationMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
		}
	}
	return slackMessages
}
