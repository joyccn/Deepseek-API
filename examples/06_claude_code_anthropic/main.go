package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	url := "http://localhost:8080/v1/messages"

	reqBody := map[string]any{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": "Hello Claude Code! Tell me what 2 + 2 is.",
			},
		},
		"thinking": map[string]any{
			"type":          "enabled",
			"budget_tokens": 1024,
		},
	}

	jsonBytes, _ := json.Marshal(reqBody)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Fatalf("Anthropic API request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("=== ANTHROPIC MESSAGES API RESPONSE ===")
	fmt.Println(string(body))
}
