package agentic_test

import (
	"strings"
	"testing"

	"deepseek-api/pkg/agentic"
)

func TestToolCallParser(t *testing.T) {
	rawLLMOutput := "Let me check the weather for you.\n<tool_call>{\"name\": \"get_weather\", \"arguments\": {\"city\": \"Jakarta\"}}</tool_call>"

	toolCalls, cleanText := agentic.ParseToolCalls(rawLLMOutput)
	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}
	if toolCalls[0].Function.Name != "get_weather" {
		t.Errorf("Expected function get_weather, got %s", toolCalls[0].Function.Name)
	}
	if strings.Contains(cleanText, "<tool_call>") {
		t.Errorf("Clean text still contains tool_call tags: %s", cleanText)
	}
}

func TestToolPromptInjection(t *testing.T) {
	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "execute_command",
				"description": "Run shell command",
			},
		},
	}
	injected := agentic.InjectToolsIntoPrompt("Hello agent!", tools)
	if !strings.Contains(injected, "execute_command") {
		t.Errorf("Injected prompt missing execute_command definition")
	}
	if !strings.Contains(injected, "<tool_call>") {
		t.Errorf("Injected prompt missing <tool_call> syntax instruction")
	}
}
