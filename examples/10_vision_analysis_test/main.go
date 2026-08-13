package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type ImageURL struct {
	URL string `json:"url"`
}

type MessageContent struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
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
	imagePath := filepath.Join("scratch", "sample_image.png")
	imgBytes, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatalf("Failed to read sample image from %s: %v", imagePath, err)
	}

	b64Image := base64.StdEncoding.EncodeToString(imgBytes)
	dataURL := fmt.Sprintf("data:image/png;base64,%s", b64Image)

	url := "http://localhost:8080/v1/chat/completions"

	promptText := "Tolong jelaskan gambar yang saya lampirkan ini: sebutkan nama karakter/hewan ini, warnanya, dan detailnya."

	reqBody := ChatCompletionRequest{
		Model: "deepseek-chat",
		Messages: []ChatMessage{
			{
				Role: "user",
				Content: []MessageContent{
					{Type: "text", Text: promptText},
					{Type: "image_url", ImageURL: &ImageURL{URL: dataURL}},
				},
			},
		},
	}

	jsonBytes, _ := json.Marshal(reqBody)

	fmt.Println("=== SENDING LOCAL IMAGE TO DEEPSEEK VISION PIPELINE VIA GOLANG SERVER ===")
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

	if len(chatResp.Choices) > 0 {
		fmt.Println("\n=== DEEPSEEK VISION EXPLANATION ===")
		fmt.Println(chatResp.Choices[0].Message.Content)
	}
}
