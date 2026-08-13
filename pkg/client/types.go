package client

type EventType int

const (
	EventThinking EventType = iota
	EventContent
	EventDone
)

type StreamEvent struct {
	Type           EventType
	Text           string
	MessageID      int
	ConversationID string
	Err            error
}

type CompletionRequest struct {
	Prompt          string `json:"prompt"`
	ChatSessionID   string `json:"chat_session_id"`
	ParentMessageID *int   `json:"parent_message_id"`
	ModelType       string `json:"model_type,omitempty"`
	ThinkingEnabled bool   `json:"thinking_enabled"`
	SearchEnabled   bool   `json:"search_enabled"`
}
