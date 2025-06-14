package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc/internal/knowledge"
	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc/internal/openai"
)

type ConversationMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

type GPTRequest struct {
	Message            string               `json:"message"`
	UserID             string               `json:"user_id"`
	ChannelID          string               `json:"channel_id"`
	MessageTS          string               `json:"message_ts"`
	ThreadTS           string               `json:"thread_ts,omitempty"`
	ConversationHistory []ConversationMessage `json:"conversation_history,omitempty"`
	CorrelationID      string               `json:"correlation_id"`
	AgentID            string               `json:"agent_id,omitempty"`
}

type GPTResponse struct {
	Response      string `json:"response"`
	CorrelationID string `json:"correlation_id"`
	Error         string `json:"error,omitempty"`
}

type Handler struct {
	openaiClient *openai.Client
	logger       *slog.Logger
	knowledge    *knowledge.Retriever
}

func NewHandler(openaiClient *openai.Client, logger *slog.Logger, knowledgeRetriever *knowledge.Retriever) *Handler {
	return &Handler{
		openaiClient: openaiClient,
		logger:       logger,
		knowledge:    knowledgeRetriever,
	}
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

func (h *Handler) handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	var req GPTRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		h.logger.Error("Empty message in request", "correlation_id", req.CorrelationID)
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

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
	if h.knowledge != nil {
		context, err := h.knowledge.GetKnowledgeContext(agentID)
		if err != nil {
			h.logger.Warn("Failed to get knowledge context", "error", err, "agent_id", agentID)
		} else if context != "" {
			knowledgeContext = context
			h.logger.Info("Added knowledge context to request", "agent_id", agentID, "context_length", len(knowledgeContext))
		}
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
