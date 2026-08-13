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
	url := "http://localhost:8080/v1/chat/completions"

	prompt := `Write a complete, high-quality Python Snake Game class with full unittest suite.
Requirements:
1. Snake class with initial length 3, direction ('UP', 'DOWN', 'LEFT', 'RIGHT'), movement logic, food spawning, wall/self collision detection, and score tracking.
2. A unittest.TestCase suite verifying:
   - Initial state (snake length, direction, score 0)
   - Movement in 4 directions
   - Eating food (growing length by 1, score +10, spawning new food)
   - Collision with wall (game over)
   - Collision with self (game over)
3. End with standard 'if __name__ == "__main__": unittest.main()' block.
Return ONLY valid executable Python code in a '''python codeblock.`

	reqBody := ChatCompletionRequest{
		Model: "deepseek-chat",
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
	}
	jsonBytes, _ := json.Marshal(reqBody)

	fmt.Println("=== REQUESTING DEEPSEEK RELAY SERVER TO GENERATE SNAKE GAME ===")
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
	fmt.Println("=== GENERATED RESPONSE FROM DEEPSEEK SERVER ===")
	fmt.Println(codeResponse)

	// Extract Python code block
	re := regexp.MustCompile("(?s)```python\n?(.*?)\n?```")
	matches := re.FindStringSubmatch(codeResponse)

	pythonCode := codeResponse
	if len(matches) >= 2 {
		pythonCode = matches[1]
	}

	outputPath := filepath.Join("scratch", "snake_game.py")
	os.MkdirAll("scratch", 0755)
	err = os.WriteFile(outputPath, []byte(pythonCode), 0644)
	if err != nil {
		log.Fatalf("Failed to write python code: %v", err)
	}

	fmt.Printf("\n✅ Generated Snake Game saved to %s\n", outputPath)
}
