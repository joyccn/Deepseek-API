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

type Server struct {
	client *client.Client
	cache  *agentic.SessionCache
}

func NewServer(c *client.Client) *Server {
	return &Server{
		client: c,
		cache:  agentic.NewSessionCache(),
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /v1/models", s.handleListModels)
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("POST /v1/messages", s.handleAnthropicMessages)
	mux.HandleFunc("POST /v1/images/generations", s.handleImageGenerations)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	now := time.Now().Unix()
	resp := map[string]any{
		"object": "list",
		"data": []map[string]any{
			{"id": "deepseek-chat", "object": "model", "created": now, "owned_by": "deepseek"},
			{"id": "deepseek-expert", "object": "model", "created": now, "owned_by": "deepseek"},
		},
		"models": []map[string]any{
			{
				"slug":                       "deepseek-chat",
				"display_name":               "DeepSeek Instant (Chat)",
				"description":                "Fast and responsive model for general coding and tasks",
				"default_reasoning_level":    "medium",
				"supported_reasoning_levels": []map[string]string{{"effort": "low"}, {"effort": "medium"}, {"effort": "high"}},
				"supported_in_api":           true,
				"context_window":             128000,
			},
			{
				"slug":                       "deepseek-expert",
				"display_name":               "DeepSeek Expert (R1)",
				"description":                "High-precision model for complex reasoning and architecture",
				"default_reasoning_level":    "high",
				"supported_reasoning_levels": []map[string]string{{"effort": "low"}, {"effort": "medium"}, {"effort": "high"}},
				"supported_in_api":           true,
				"context_window":             128000,
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":{"message":"Invalid JSON payload: %v"}}`, err), http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		http.Error(w, `{"error":{"message":"messages array must not be empty"}}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Build prompt from messages
	var promptBuilder strings.Builder
	for _, m := range req.Messages {
		if m.Role != "" {
			promptBuilder.WriteString(m.Role)
			promptBuilder.WriteString(": ")
		}
		promptBuilder.WriteString(m.Content)
		promptBuilder.WriteString("\n")
	}
	prompt := promptBuilder.String()

	// Inject tool instructions if tools array is provided
	if len(req.Tools) > 0 {
		prompt = agentic.InjectToolsIntoPrompt(prompt, req.Tools)
	}

	// Resolve session ID
	sessionID := req.ConversationID
	if sessionID == "" {
		newID, err := s.client.CreateChatSession(ctx)
		if err != nil {
			slog.Error("Failed to create chat session", "error", err)
			http.Error(w, fmt.Sprintf(`{"error":{"message":"Failed to initialize DeepSeek session: %v"}}`, err), http.StatusInternalServerError)
			return
		}
		sessionID = newID
	}

	// Fetch PoW Header
	powHeader, err := s.client.FetchPoWHeader(ctx)
	if err != nil {
		slog.Error("Failed to solve PoW header", "error", err)
		http.Error(w, fmt.Sprintf(`{"error":{"message":"PoW solver failure: %v"}}`, err), http.StatusInternalServerError)
		return
	}

	modelType := "default"
	if req.Model == "deepseek-expert" {
		modelType = "expert"
	}

	compReq := client.CompletionRequest{
		Prompt:          prompt,
		ChatSessionID:   sessionID,
		ModelType:       modelType,
		ThinkingEnabled: req.Thinking,
		SearchEnabled:   req.Search,
	}

	if req.Stream {
		s.handleStreamResponse(w, r, compReq, powHeader, req.Model, sessionID)
		return
	}

	s.handleNonStreamResponse(w, compReq, powHeader, req.Model, sessionID)
}

func (s *Server) handleStreamResponse(w http.ResponseWriter, r *http.Request, compReq client.CompletionRequest, powHeader, model, sessionID string) {
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
		slog.Error("Stream initialization error", "error", err)
		return
	}

	created := time.Now().Unix()
	reqID := fmt.Sprintf("chatcmpl-%d", created)
	var fullTextBuilder strings.Builder

	for ev := range events {
		var chunk ChatCompletionChunk
		chunk.ID = reqID
		chunk.Object = "chat.completion.chunk"
		chunk.Created = created
		chunk.Model = model
		chunk.ConversationID = sessionID

		switch ev.Type {
		case client.EventThinking:
			chunk.Choices = []ChatChunkChoice{
				{
					Index: 0,
					Delta: ChatChoiceDelta{
						ReasoningContent: ev.Text,
					},
				},
			}
		case client.EventContent:
			fullTextBuilder.WriteString(ev.Text)
			chunk.Choices = []ChatChunkChoice{
				{
					Index: 0,
					Delta: ChatChoiceDelta{
						Content: ev.Text,
					},
				},
			}
		}

		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	fullText := fullTextBuilder.String()
	toolCalls, _ := agentic.ParseToolCalls(fullText)

	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	doneChunk := ChatCompletionChunk{
		ID:             reqID,
		Object:         "chat.completion.chunk",
		Created:        created,
		Model:          model,
		ConversationID: sessionID,
		Choices: []ChatChunkChoice{
			{
				Index: 0,
				Delta: ChatChoiceDelta{
					ToolCalls: toolCalls,
				},
				FinishReason: &finishReason,
			},
		},
	}
	doneData, _ := json.Marshal(doneChunk)
	fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", doneData)
	flusher.Flush()
}

func (s *Server) handleNonStreamResponse(w http.ResponseWriter, compReq client.CompletionRequest, powHeader, model, sessionID string) {
	events, err := s.client.Stream(context.Background(), compReq, powHeader)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":{"message":"Request error: %v"}}`, err), http.StatusInternalServerError)
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
	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	created := time.Now().Unix()
	resp := ChatCompletionResponse{
		ID:             fmt.Sprintf("chatcmpl-%d", created),
		Object:         "chat.completion",
		Created:        created,
		Model:          model,
		ConversationID: sessionID,
		Choices: []ChatResponseChoice{
			{
				Index: 0,
				Message: ChatResponseMessage{
					Role:             "assistant",
					Content:          finalContent,
					ReasoningContent: finalThinking,
					ToolCalls:        toolCalls,
				},
				FinishReason: finishReason,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
