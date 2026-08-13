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
	"strings"
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
	Model          string        `json:"model"`
	Messages       []ChatMessage `json:"messages"`
	ConversationID string        `json:"conversation_id,omitempty"`
	Thinking       bool          `json:"thinking,omitempty"`
	Search         bool          `json:"search,omitempty"`
}

type ChatCompletionResponse struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	Choices        []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content,omitempty"`
		} `json:"message"`
	} `json:"choices"`
}

func sendRequest(url string, reqBody ChatCompletionRequest) (string, string, string, error) {
	jsonBytes, _ := json.Marshal(reqBody)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return "", "", "", err
	}

	content := ""
	reasoning := ""
	if len(chatResp.Choices) > 0 {
		content = chatResp.Choices[0].Message.Content
		reasoning = chatResp.Choices[0].Message.ReasoningContent
	}
	return content, reasoning, chatResp.ConversationID, nil
}

func main() {
	imagePath := filepath.Join("scratch", "sample_image.png")
	imgBytes, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatalf("Failed to read sample image: %v", err)
	}

	b64Image := base64.StdEncoding.EncodeToString(imgBytes)
	dataURL := fmt.Sprintf("data:image/png;base64,%s", b64Image)

	url := "http://localhost:8080/v1/chat/completions"

	fmt.Println("=== TURN 1: VISION + EXPERT (R1) + SEARCH + DEEPTHINK ===")
	prompt1 := "Tolong jelaskan secara detail gambar ini menggunakan analisis mendalam R1 dan pencarian web."

	history := []ChatMessage{
		{
			Role: "user",
			Content: []MessageContent{
				{Type: "text", Text: prompt1},
				{Type: "image_url", ImageURL: &ImageURL{URL: dataURL}},
			},
		},
	}

	req1 := ChatCompletionRequest{
		Model:    "deepseek-expert",
		Messages: history,
		Thinking: true,
		Search:   true,
	}

	content1, reasoning1, convID, err := sendRequest(url, req1)
	if err != nil {
		log.Fatalf("Turn 1 failed: %v", err)
	}

	fmt.Printf("[Turn 1] ConvID: %s\n", convID)
	if reasoning1 != "" {
		fmt.Printf("[Turn 1] DeepThink R1 Reasoning:\n%s\n\n", strings.TrimSpace(reasoning1))
	}
	fmt.Printf("[Turn 1] Response Content:\n%s\n\n", strings.TrimSpace(content1))

	// Turn 2: Follow-up question using the SAME ConversationID
	fmt.Println("=== TURN 2: CONTINUING SAME CONVERSATION (FOLLOW-UP QUESTION) ===")
	prompt2 := "Berdasarkan gambar tadi yang sudah kamu analisa, sebutkan 3 kesimpulan paling penting dari gambar tersebut!"

	history = append(history,
		ChatMessage{Role: "assistant", Content: content1},
		ChatMessage{Role: "user", Content: prompt2},
	)

	req2 := ChatCompletionRequest{
		Model:          "deepseek-expert",
		Messages:       history,
		ConversationID: convID,
		Thinking:       true,
		Search:         true,
	}

	content2, reasoning2, convID2, err := sendRequest(url, req2)
	if err != nil {
		log.Fatalf("Turn 2 failed: %v", err)
	}

	fmt.Printf("[Turn 2] ConvID: %s (Matches Turn 1: %v)\n", convID2, convID2 == convID)
	if reasoning2 != "" {
		fmt.Printf("[Turn 2] DeepThink R1 Reasoning:\n%s\n\n", strings.TrimSpace(reasoning2))
	}
	fmt.Printf("[Turn 2] Response Content:\n%s\n\n", strings.TrimSpace(content2))

	fmt.Println("✅ MULTI-TURN VISION + EXPERT (R1) + SEARCH + DEEPTHINK TEST COMPLETED SUCCESSFULLY!")
}
