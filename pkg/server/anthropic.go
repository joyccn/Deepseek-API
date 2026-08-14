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

	var agenticMsgs []agentic.Message

	// Add system prompt if present
	if req.System != nil {
		switch sys := req.System.(type) {
		case string:
			agenticMsgs = append(agenticMsgs, agentic.Message{Role: "system", Content: sys})
		case []any:
			var sysTexts []string
			for _, item := range sys {
				if itemMap, ok := item.(map[string]any); ok {
					if txt, ok := itemMap["text"].(string); ok {
						sysTexts = append(sysTexts, txt)
					}
				}
			}
			if len(sysTexts) > 0 {
				agenticMsgs = append(agenticMsgs, agentic.Message{Role: "system", Content: strings.Join(sysTexts, "\n")})
			}
		}
	}

	for _, m := range req.Messages {
		switch c := m.Content.(type) {
		case string:
			agenticMsgs = append(agenticMsgs, agentic.Message{Role: m.Role, Content: c})
		case []any:
			var textParts []string
			var toolCalls []agentic.ToolCall
			for _, item := range c {
				if itemMap, ok := item.(map[string]any); ok {
					bType, _ := itemMap["type"].(string)
					switch bType {
					case "text":
						if txt, ok := itemMap["text"].(string); ok {
							textParts = append(textParts, txt)
						}
					case "tool_result":
						toolUseID, _ := itemMap["tool_use_id"].(string)
						content := itemMap["content"]
						var contentStr string
						switch cv := content.(type) {
						case string:
							contentStr = cv
						default:
							b, _ := json.Marshal(cv)
							contentStr = string(b)
						}
						agenticMsgs = append(agenticMsgs, agentic.Message{
							Role:       "tool",
							Content:    contentStr,
							ToolCallID: toolUseID,
							Name:       toolUseID,
						})
					case "tool_use":
						toolID, _ := itemMap["id"].(string)
						name, _ := itemMap["name"].(string)
						inputBytes, _ := json.Marshal(itemMap["input"])
						toolCalls = append(toolCalls, agentic.ToolCall{
							ID:   toolID,
							Type: "function",
							Function: agentic.FunctionCall{
								Name:      name,
								Arguments: string(inputBytes),
							},
						})
					case "thinking":
						if thinkText, ok := itemMap["thinking"].(string); ok {
							textParts = append(textParts, fmt.Sprintf("<think>%s</think>", thinkText))
						}
					}
				}
			}
			if len(textParts) > 0 || len(toolCalls) > 0 {
				agenticMsgs = append(agenticMsgs, agentic.Message{
					Role:      m.Role,
					Content:   strings.Join(textParts, "\n"),
					ToolCalls: toolCalls,
				})
			}
		}
	}

	prompt := agentic.BuildTrajectoryPrompt(agenticMsgs, req.Tools)

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

	modelLower := strings.ToLower(req.Model)
	thinkingEnabled := (req.Thinking != nil && req.Thinking.Type == "enabled") ||
		strings.Contains(modelLower, "reasoner") ||
		strings.Contains(modelLower, "r1") ||
		strings.Contains(modelLower, "expert") ||
		strings.Contains(modelLower, "sonnet-3-7") ||
		strings.Contains(modelLower, "claude-3-7")

	compReq := client.CompletionRequest{
		Prompt:          prompt,
		ChatSessionID:   sessionID,
		ModelType:       "default",
		ThinkingEnabled: thinkingEnabled,
		SearchEnabled:   false,
	}

	if req.Stream {
		s.handleAnthropicStreamResponse(w, r, compReq, powHeader, req.Model, sessionID)
		return
	}

	s.handleAnthropicNonStreamResponse(w, r.Context(), compReq, powHeader, req.Model, sessionID)
}

func (s *Server) handleAnthropicStreamResponse(w http.ResponseWriter, r *http.Request, compReq client.CompletionRequest, powHeader, model, sessionID string) {
	defer s.client.DeleteChatSession(context.Background(), sessionID)
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
		blockIndex++
	}

	fullText := fullTextBuilder.String()
	toolCalls, _ := agentic.ParseToolCalls(fullText)

	stopReason := "end_turn"
	if len(toolCalls) > 0 {
		stopReason = "tool_use"
		for _, tc := range toolCalls {
			var inputObj any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputObj); err != nil {
				inputObj = map[string]any{}
			}

			tbStart, _ := json.Marshal(map[string]any{
				"type":  "content_block_start",
				"index": blockIndex,
				"content_block": map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": inputObj,
				},
			})
			fmt.Fprintf(w, "event: content_block_start\ndata: %s\n\n", tbStart)

			tbDelta, _ := json.Marshal(map[string]any{
				"type":  "content_block_delta",
				"index": blockIndex,
				"delta": map[string]string{
					"type":         "input_json_delta",
					"partial_json": tc.Function.Arguments,
				},
			})
			fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", tbDelta)

			tbStop, _ := json.Marshal(map[string]any{
				"type":  "content_block_stop",
				"index": blockIndex,
			})
			fmt.Fprintf(w, "event: content_block_stop\ndata: %s\n\n", tbStop)
			flusher.Flush()
			blockIndex++
		}
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

func (s *Server) handleAnthropicNonStreamResponse(w http.ResponseWriter, ctx context.Context, compReq client.CompletionRequest, powHeader, model, sessionID string) {
	defer s.client.DeleteChatSession(context.Background(), sessionID)
	events, err := s.client.Stream(ctx, compReq, powHeader)
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
			var inputObj any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputObj); err != nil {
				inputObj = map[string]any{}
			}
			contents = append(contents, AnthropicMessageContent{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: inputObj,
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
