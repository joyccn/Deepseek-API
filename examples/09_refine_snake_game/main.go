package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	url := "http://localhost:" + port + "/v1/chat/completions"

	prompt := `Fix and refine the Python Snake Game class and unit tests so that 100% of unittest cases pass without error.
Make sure:
1. Direction changes cannot reverse 180 degrees directly (e.g. RIGHT to LEFT is ignored). In unit tests, to turn LEFT from RIGHT, first change to UP or DOWN, then move.
2. All 14 unit tests pass 100% cleanly.
Return ONLY valid executable Python code in a '''python codeblock.`

	reqBody := ChatCompletionRequest{
		Model: "deepseek-chat",
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
	}
	jsonBytes, _ := json.Marshal(reqBody)

	fmt.Println("=== SENDING REFINEMENT REQUEST TO DEEPSEEK SERVER ===")
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Fatalf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("HTTP %d error: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		log.Fatalf("JSON decode error: %v", err)
	}

	codeResponse := chatResp.Choices[0].Message.Content
	re := regexp.MustCompile("(?s)```python\n?(.*?)\n?```")
	matches := re.FindStringSubmatch(codeResponse)

	pythonCode := codeResponse
	if len(matches) >= 2 {
		pythonCode = matches[1]
	}

	outputPath := filepath.Join("scratch", "snake_game_fixed.py")
	os.MkdirAll("scratch", 0755)
	err = os.WriteFile(outputPath, []byte(pythonCode), 0644)
	if err != nil {
		log.Fatalf("Failed to write python code: %v", err)
	}

	fmt.Printf("\n✅ Refined Snake Game saved to %s\n", outputPath)
}
