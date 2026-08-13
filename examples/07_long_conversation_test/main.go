package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model          string        `json:"model"`
	Messages       []ChatMessage `json:"messages"`
	ConversationID string        `json:"conversation_id,omitempty"`
}

type ChatCompletionResponse struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	Choices        []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func sendChatMessages(url, model string, messages []ChatMessage, convID string) (string, string, error) {
	reqBody := ChatCompletionRequest{
		Model:          model,
		Messages:       messages,
		ConversationID: convID,
	}
	jsonBytes, _ := json.Marshal(reqBody)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return "", "", err
	}

	content := ""
	if len(chatResp.Choices) > 0 {
		content = chatResp.Choices[0].Message.Content
	}
	return content, chatResp.ConversationID, nil
}

func main() {
	url := "http://localhost:8080/v1/chat/completions"

	fmt.Println("=== TEST 1: STARTING MULTI-TURN CONTINUOUS CONVERSATION ===")
	secretKeyword := "ANTIGRAVITY-GOLANG-2026"
	prompt1 := fmt.Sprintf("Please remember this secret keyword: %s", secretKeyword)

	history := []ChatMessage{
		{Role: "user", Content: prompt1},
	}

	reply1, convID, err := sendChatMessages(url, "deepseek-chat", history, "")
	if err != nil {
		log.Fatalf("Turn 1 failed: %v", err)
	}
	fmt.Printf("[Turn 1] ConvID: %s\n", convID)
	fmt.Printf("[Turn 1] Reply: %s\n\n", strings.TrimSpace(reply1))

	// Append assistant response and new user question to message history
	history = append(history,
		ChatMessage{Role: "assistant", Content: reply1},
		ChatMessage{Role: "user", Content: "What was the secret keyword I asked you to remember?"},
	)

	// Turn 2: Send accumulated message history with same ConversationID
	fmt.Println("=== TEST 2: CONTINUING CONVERSATION (ACCUMULATED MESSAGES) ===")
	reply2, convID2, err := sendChatMessages(url, "deepseek-chat", history, convID)
	if err != nil {
		log.Fatalf("Turn 2 failed: %v", err)
	}
	fmt.Printf("[Turn 2] ConvID: %s (Matches Turn 1: %v)\n", convID2, convID2 == convID)
	fmt.Printf("[Turn 2] Reply: %s\n\n", strings.TrimSpace(reply2))

	if strings.Contains(reply2, secretKeyword) {
		fmt.Println("✅ SUCCESS: Conversation context remembered across turns!")
	} else {
		fmt.Println("⚠️ WARNING: Secret keyword not found in response.")
	}

	// Turn 3: Start a NEW Conversation Session (Reset / Clean State)
	fmt.Println("\n=== TEST 3: NEW CONVERSATION SESSION (RESET / NEW) ===")
	newHistory := []ChatMessage{
		{Role: "user", Content: "What was the secret keyword I asked you to remember earlier?"},
	}
	reply3, newConvID, err := sendChatMessages(url, "deepseek-chat", newHistory, "new")
	if err != nil {
		log.Fatalf("Turn 3 failed: %v", err)
	}
	fmt.Printf("[Turn 3] New ConvID: %s (Different from Turn 1: %v)\n", newConvID, newConvID != convID)
	fmt.Printf("[Turn 3] Reply: %s\n\n", strings.TrimSpace(reply3))

	if !strings.Contains(reply3, secretKeyword) {
		fmt.Println("✅ SUCCESS: New conversation session created, clean state verified!")
	}
}
