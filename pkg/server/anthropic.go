package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"deepseek-api/pkg/agentic"
	"deepseek-api/pkg/client"
)

// Anthropic Request Schemas
type AnthropicMessageContent struct {
	Type      string `json:"type,omitempty"`
	Text      string `json:"text,omitempty"`
	Thinking  string `json:"thinking,omitempty"`
	ImageURL  string `json:"image_url,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // can be string or []AnthropicMessageContent
}

type AnthropicThinkingOption struct {
	Type         string `json:"type"` // "enabled"
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

type AnthropicRequest struct {
	Model       string                   `json:"model"`
	Messages    []AnthropicMessage       `json:"messages"`
	System      any                      `json:"system,omitempty"` // string or []AnthropicMessageContent
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
	Thinking    *AnthropicThinkingOption `json:"thinking,omitempty"`
	Tools       []any                    `json:"tools,omitempty"`
	Temperature *float64                 `json:"temperature,omitempty"`
}

// Anthropic Response Schemas
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type AnthropicResponse struct {
	ID           string                    `json:"id"`
	Type         string                    `json:"type"` // "message"
	Role         string                    `json:"role"` // "assistant"
	Model        string                    `json:"model"`
	Content      []AnthropicMessageContent `json:"content"`
	StopReason   string                    `json:"stop_reason"` // "end_turn" or "tool_use"
	StopSequence *string                   `json:"stop_sequence"`
	Usage        AnthropicUsage            `json:"usage"`
}

func (s *Server) handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	var req AnthropicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":{"type":"invalid_request_error","message":"Invalid JSON: %v"}}`, err), http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		http.Error(w, `{"error":{"type":"invalid_request_error","message":"messages must not be empty"}}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Extract prompt from system & messages
	var promptBuilder strings.Builder

	// Add system prompt if present
	if req.System != nil {
		switch sys := req.System.(type) {
		case string:
			promptBuilder.WriteString("System: ")
			promptBuilder.WriteString(sys)
			promptBuilder.WriteString("\n")
		}
	}

	for _, m := range req.Messages {
		promptBuilder.WriteString(m.Role)
		promptBuilder.WriteString(": ")
		switch c := m.Content.(type) {
		case string:
			promptBuilder.WriteString(c)
		case []any:
			for _, item := range c {
				if itemMap, ok := item.(map[string]any); ok {
					if txt, ok := itemMap["text"].(string); ok {
						promptBuilder.WriteString(txt)
					}
				}
			}
		}
		promptBuilder.WriteString("\n")
	}

	prompt := promptBuilder.String()

	// Inject tool instructions if tools are present
	if len(req.Tools) > 0 {
		prompt = agentic.InjectToolsIntoPrompt(prompt, req.Tools)
	}

	// Create Chat Session
	sessionID, err := s.client.CreateChatSession(ctx)
	if err != nil {
		slog.Error("Failed to create chat session for Anthropic request", "error", err)
		http.Error(w, fmt.Sprintf(`{"error":{"type":"api_error","message":"Failed to initialize DeepSeek session: %v"}}`, err), http.StatusInternalServerError)
		return
	}

	// Fetch PoW Header
	powHeader, err := s.client.FetchPoWHeader(ctx)
	if err != nil {
		slog.Error("Failed to solve PoW header", "error", err)
		http.Error(w, fmt.Sprintf(`{"error":{"type":"api_error","message":"PoW solver failure: %v"}}`, err), http.StatusInternalServerError)
		return
	}

	thinkingEnabled := req.Thinking != nil && req.Thinking.Type == "enabled"

	compReq := client.CompletionRequest{
		Prompt:          prompt,
		ChatSessionID:   sessionID,
		ModelType:       "default",
		ThinkingEnabled: thinkingEnabled,
		SearchEnabled:   false,
	}

	if req.Stream {
		s.handleAnthropicStreamResponse(w, r, compReq, powHeader, req.Model)
		return
	}

	s.handleAnthropicNonStreamResponse(w, compReq, powHeader, req.Model)
}

func (s *Server) handleAnthropicStreamResponse(w http.ResponseWriter, r *http.Request, compReq client.CompletionRequest, powHeader, model string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	events, err := s.client.Stream(r.Context(), compReq, powHeader)
	if err != nil {
		slog.Error("Anthropic stream initialization error", "error", err)
		return
	}

	msgID := fmt.Sprintf("msg_%d", time.Now().Unix())

	// 1. Send message_start
	msgStartData, _ := json.Marshal(map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            msgID,
			"type":          "message",
			"role":          "assistant",
			"content":       []any{},
			"model":         model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]int{"input_tokens": 10, "output_tokens": 0},
		},
	})
	fmt.Fprintf(w, "event: message_start\ndata: %s\n\n", msgStartData)
	flusher.Flush()

	// 2. Stream content blocks
	blockIndex := 0
	hasStartedThinkingBlock := false
	hasStartedTextBlock := false

	var fullTextBuilder strings.Builder

	for ev := range events {
		switch ev.Type {
		case client.EventThinking:
			if !hasStartedThinkingBlock {
				hasStartedThinkingBlock = true
				startData, _ := json.Marshal(map[string]any{
					"type":          "content_block_start",
					"index":         blockIndex,
					"content_block": map[string]string{"type": "thinking", "thinking": ""},
				})
				fmt.Fprintf(w, "event: content_block_start\ndata: %s\n\n", startData)
				flusher.Flush()
			}
			deltaData, _ := json.Marshal(map[string]any{
				"type":  "content_block_delta",
				"index": blockIndex,
				"delta": map[string]string{"type": "thinking_delta", "thinking": ev.Text},
			})
			fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", deltaData)
			flusher.Flush()
		case client.EventContent:
			if hasStartedThinkingBlock {
				// Close thinking block
				stopData, _ := json.Marshal(map[string]any{
					"type":  "content_block_stop",
					"index": blockIndex,
				})
				fmt.Fprintf(w, "event: content_block_stop\ndata: %s\n\n", stopData)
				flusher.Flush()
				hasStartedThinkingBlock = false
				blockIndex++
			}

			if !hasStartedTextBlock {
				hasStartedTextBlock = true
				startData, _ := json.Marshal(map[string]any{
					"type":          "content_block_start",
					"index":         blockIndex,
					"content_block": map[string]string{"type": "text", "text": ""},
				})
				fmt.Fprintf(w, "event: content_block_start\ndata: %s\n\n", startData)
				flusher.Flush()
			}

			fullTextBuilder.WriteString(ev.Text)
			deltaData, _ := json.Marshal(map[string]any{
				"type":  "content_block_delta",
				"index": blockIndex,
				"delta": map[string]string{"type": "text_delta", "text": ev.Text},
			})
			fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", deltaData)
			flusher.Flush()
		}
	}

	if hasStartedThinkingBlock || hasStartedTextBlock {
		stopData, _ := json.Marshal(map[string]any{
			"type":  "content_block_stop",
			"index": blockIndex,
		})
		fmt.Fprintf(w, "event: content_block_stop\ndata: %s\n\n", stopData)
		flusher.Flush()
	}

	fullText := fullTextBuilder.String()
	toolCalls, _ := agentic.ParseToolCalls(fullText)

	stopReason := "end_turn"
	if len(toolCalls) > 0 {
		stopReason = "tool_use"
	}

	// 3. Send message_delta & message_stop
	msgDeltaData, _ := json.Marshal(map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": map[string]int{"output_tokens": 20},
	})
	fmt.Fprintf(w, "event: message_delta\ndata: %s\n\n", msgDeltaData)

	msgStopData, _ := json.Marshal(map[string]any{
		"type": "message_stop",
	})
	fmt.Fprintf(w, "event: message_stop\ndata: %s\n\n", msgStopData)
	flusher.Flush()
}

func (s *Server) handleAnthropicNonStreamResponse(w http.ResponseWriter, compReq client.CompletionRequest, powHeader, model string) {
	events, err := s.client.Stream(context.Background(), compReq, powHeader)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":{"type":"api_error","message":"Request error: %v"}}`, err), http.StatusInternalServerError)
		return
	}

	var thinkBuilder, contentBuilder strings.Builder
	for ev := range events {
		switch ev.Type {
		case client.EventThinking:
			thinkBuilder.WriteString(ev.Text)
		case client.EventContent:
			contentBuilder.WriteString(ev.Text)
		}
	}

	rawContent := contentBuilder.String()
	extractedThinking, cleanContent := agentic.ExtractThinkingContent(rawContent)

	finalThinking := thinkBuilder.String()
	if finalThinking == "" && extractedThinking != "" {
		finalThinking = extractedThinking
	}

	toolCalls, finalContent := agentic.ParseToolCalls(cleanContent)

	var contents []AnthropicMessageContent
	if finalThinking != "" {
		contents = append(contents, AnthropicMessageContent{
			Type:     "thinking",
			Thinking: finalThinking,
		})
	}

	if finalContent != "" {
		contents = append(contents, AnthropicMessageContent{
			Type: "text",
			Text: finalContent,
		})
	}

	stopReason := "end_turn"
	if len(toolCalls) > 0 {
		stopReason = "tool_use"
		for _, tc := range toolCalls {
			contents = append(contents, AnthropicMessageContent{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: tc.Function.Arguments,
			})
		}
	}

	resp := AnthropicResponse{
		ID:         fmt.Sprintf("msg_%d", time.Now().Unix()),
		Type:       "message",
		Role:       "assistant",
		Model:      model,
		Content:    contents,
		StopReason: stopReason,
		Usage: AnthropicUsage{
			InputTokens:  10,
			OutputTokens: 20,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
