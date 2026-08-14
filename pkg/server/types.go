package server

import "deepseek-api/pkg/agentic"

type ChatMessage struct {
	Role       string             `json:"role"`
	Content    any                `json:"content"`
	ToolCalls  []agentic.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string             `json:"tool_call_id,omitempty"`
	Name       string             `json:"name,omitempty"`
}

type ChatCompletionRequest struct {
	Model           string        `json:"model"`
	Messages        []ChatMessage `json:"messages"`
	Stream          bool          `json:"stream"`
	ConversationID  string        `json:"conversation_id,omitempty"`
	Thinking        bool          `json:"thinking,omitempty"`
	ThinkingEnabled bool          `json:"thinking_enabled,omitempty"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
	Search          bool          `json:"search,omitempty"`
	SearchEnabled   bool          `json:"search_enabled,omitempty"`
	WebSearch       bool          `json:"web_search,omitempty"`
	Tools           []any         `json:"tools,omitempty"`
}

type ChatChoiceDelta struct {
	Role             string             `json:"role,omitempty"`
	Content          string             `json:"content,omitempty"`
	ReasoningContent string             `json:"reasoning_content,omitempty"`
	ToolCalls        []agentic.ToolCall `json:"tool_calls,omitempty"`
}

type ChatChunkChoice struct {
	Index        int             `json:"index"`
	Delta        ChatChoiceDelta `json:"delta"`
	FinishReason *string         `json:"finish_reason"`
}

type ChatCompletionChunk struct {
	ID             string            `json:"id"`
	Object         string            `json:"object"`
	Created        int64             `json:"created"`
	Model          string            `json:"model"`
	ConversationID string            `json:"conversation_id,omitempty"`
	Choices        []ChatChunkChoice `json:"choices"`
}

type ChatResponseMessage struct {
	Role             string             `json:"role"`
	Content          string             `json:"content"`
	ReasoningContent string             `json:"reasoning_content,omitempty"`
	ToolCalls        []agentic.ToolCall `json:"tool_calls,omitempty"`
}

type ChatResponseChoice struct {
	Index        int                 `json:"index"`
	Message      ChatResponseMessage `json:"message"`
	FinishReason string              `json:"finish_reason"`
}

type ChatCompletionResponse struct {
	ID             string               `json:"id"`
	Object         string               `json:"object"`
	Created        int64                `json:"created"`
	Model          string               `json:"model"`
	ConversationID string               `json:"conversation_id"`
	Choices        []ChatResponseChoice `json:"choices"`
}
