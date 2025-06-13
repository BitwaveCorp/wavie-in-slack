package slack

import (
	"time"
)

// BroadcastRequest represents a request to broadcast a conversation to the monitoring channel
type BroadcastRequest struct {
	UserID        string    `json:"user_id"`
	ChannelID     string    `json:"channel_id"`
	ThreadID      string    `json:"thread_id"`
	Question      string    `json:"question"`
	Response      string    `json:"response"`
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID string    `json:"correlation_id"`
}

// FeedbackRequest represents a request to send feedback to the monitoring channel
type FeedbackRequest struct {
	UserID        string    `json:"user_id"`
	ChannelID     string    `json:"channel_id"`
	MessageTS     string    `json:"message_ts"`
	ThreadTS      string    `json:"thread_ts,omitempty"`
	FeedbackType  string    `json:"feedback_type"` // "positive", "negative", or "text"
	FeedbackText  string    `json:"feedback_text,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID string    `json:"correlation_id"`
}

// SlackMessage represents a message to be sent to Slack
type SlackMessage struct {
	Channel string         `json:"channel"`
	Text    string         `json:"text,omitempty"`
	Blocks  []MessageBlock `json:"blocks,omitempty"`
}

// MessageBlock represents a block in a Slack message
type MessageBlock struct {
	Type string      `json:"type"`
	Text *TextObject `json:"text,omitempty"`
}

// TextObject represents text in a Slack message block
type TextObject struct {
	Type string `json:"type"` // "plain_text" or "mrkdwn"
	Text string `json:"text"`
}
