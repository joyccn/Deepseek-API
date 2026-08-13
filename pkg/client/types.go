package client

type StreamEventType string

const (
	EventThinking StreamEventType = "THINKING"
	EventContent  StreamEventType = "CONTENT"
)

type StreamEvent struct {
	Type StreamEventType
	Text string
}

type CompletionRequest struct {
	Prompt          string   `json:"prompt"`
	ChatSessionID   string   `json:"chat_session_id"`
	ParentMessageID any      `json:"parent_message_id"`
	ModelType       string   `json:"model_type,omitempty"`
	ThinkingEnabled bool     `json:"thinking_enabled"`
	SearchEnabled   bool     `json:"search_enabled"`
	RefFileIDs      []string `json:"ref_file_ids,omitempty"`
}
