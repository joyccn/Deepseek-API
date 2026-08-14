package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"deepseek-api/pkg/agentic"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []any         `json:"tools,omitempty"`
}

type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role             string             `json:"role"`
			Content          string             `json:"content"`
			ReasoningContent string             `json:"reasoning_content,omitempty"`
			ToolCalls        []agentic.ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	url := "http://localhost:" + port + "/v1/chat/completions"

	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "get_weather",
				"description": "Fetch current weather for a city",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{"type": "string", "description": "City name"},
					},
					"required": []string{"city"},
				},
			},
		},
	}

	reqBody := ChatCompletionRequest{
		Model: "deepseek-chat",
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: "What is the weather in Jakarta right now? Please call the get_weather tool.",
			},
		},
		Tools: tools,
	}

	jsonBytes, err := json.MarshalIndent(reqBody, "", "  ")
	if err != nil {
		log.Fatalf("JSON marshal error: %v", err)
	}

	fmt.Println("=== 🚀 SENDING REAL TOOL CALLING REQUEST TO DEEPSEEK SERVER ===")
	fmt.Printf("POST %s\n\nPayload:\n%s\n\n", url, string(jsonBytes))

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Fatalf("API request failed (is server running?): %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Read response failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("HTTP %d error: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		log.Fatalf("Unmarshal error: %v, raw: %s", err, string(bodyBytes))
	}

	fmt.Println("=== 📥 RAW SERVER RESPONSE ===")
	fmt.Println(string(bodyBytes))

	fmt.Println("\n=== 🎯 PARSED RESULT ===")
	if len(chatResp.Choices) > 0 {
		choice := chatResp.Choices[0]
		fmt.Printf("Finish Reason: %s\n", choice.FinishReason)
		fmt.Printf("Content: %s\n", choice.Message.Content)
		if len(choice.Message.ToolCalls) > 0 {
			fmt.Printf("✅ Detected %d Tool Call(s):\n", len(choice.Message.ToolCalls))
			for i, tc := range choice.Message.ToolCalls {
				fmt.Printf("  [%d] ID: %s | Function: %s | Arguments: %s\n", i+1, tc.ID, tc.Function.Name, tc.Function.Arguments)
			}
		} else {
			fmt.Println("ℹ️ No tool calls returned by model.")
		}
	}
}

