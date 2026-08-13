package main

import (
	"fmt"
	"deepseek-api/pkg/agentic"
)

func main() {
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

	prompt := "What is the weather in Jakarta right now?"
	injectedPrompt := agentic.InjectToolsIntoPrompt(prompt, tools)

	fmt.Println("=== INJECTED PROMPT FOR DEEPSEEK ===")
	fmt.Println(injectedPrompt)

	// Simulated LLM response containing tool call
	llmResponse := "I'll check the weather for Jakarta.\n<tool_call>{\"name\": \"get_weather\", \"arguments\": {\"city\": \"Jakarta\"}}</tool_call>"

	toolCalls, cleanText := agentic.ParseToolCalls(llmResponse)

	fmt.Println("\n=== PARSED RESULT ===")
	fmt.Printf("Clean Text: %s\n", cleanText)
	for i, call := range toolCalls {
		fmt.Printf("Tool Call [%d]: ID=%s, Function=%s, Arguments=%s\n", i+1, call.ID, call.Function.Name, call.Function.Arguments)
	}
}
