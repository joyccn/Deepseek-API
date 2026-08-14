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

func TestMultilineToolCallParser(t *testing.T) {
	rawLLMOutput := `Here is the command execution:
<tool_call>
{
  "name": "bash",
  "input": {
    "command": "ls -la"
  }
}
</tool_call>
Done.`

	toolCalls, cleanText := agentic.ParseToolCalls(rawLLMOutput)
	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call from multiline, got %d", len(toolCalls))
	}
	if toolCalls[0].Function.Name != "bash" {
		t.Errorf("Expected function bash, got %s", toolCalls[0].Function.Name)
	}
	if !strings.Contains(toolCalls[0].Function.Arguments, "ls -la") {
		t.Errorf("Expected argument to contain 'ls -la', got %s", toolCalls[0].Function.Arguments)
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
	if !strings.Contains(injected, "<tool_call>") && !strings.Contains(injected, "<tool>") {
		t.Errorf("Injected prompt missing tool calling syntax instruction")
	}
}

func TestMultiFormatToolTags(t *testing.T) {
	// 1. Tag suffix format: <tool:bash>{"command": "whoami"}</tool>
	raw1 := `I will run whoami. <tool:bash>{"command": "whoami"}</tool>`
	calls1, clean1 := agentic.ParseToolCalls(raw1)
	if len(calls1) != 1 || calls1[0].Function.Name != "bash" {
		t.Errorf("Failed parsing <tool:bash> tag: %+v", calls1)
	}
	if !strings.Contains(calls1[0].Function.Arguments, "whoami") {
		t.Errorf("Expected whoami argument, got %s", calls1[0].Function.Arguments)
	}
	if strings.Contains(clean1, "<tool:bash>") {
		t.Errorf("Clean text still has tag: %s", clean1)
	}

	// 2. Tool attribute format: <tool name="calculator">{"expr": "10*5"}</tool>
	raw2 := `<tool name="calculator">{"expr": "10*5"}</tool>`
	calls2, _ := agentic.ParseToolCalls(raw2)
	if len(calls2) != 1 || calls2[0].Function.Name != "calculator" {
		t.Errorf("Failed parsing <tool name='calculator'>: %+v", calls2)
	}

	// 3. XML Parameter format: <tool:write><parameter name="path">app.go</parameter></tool>
	raw3 := `<tool:write><parameter name="path">app.go</parameter></tool>`
	calls3, _ := agentic.ParseToolCalls(raw3)
	if len(calls3) != 1 || calls3[0].Function.Name != "write" {
		t.Errorf("Failed parsing parameter XML tag: %+v", calls3)
	}
	if !strings.Contains(calls3[0].Function.Arguments, "app.go") {
		t.Errorf("Expected app.go argument, got %s", calls3[0].Function.Arguments)
	}
}

func TestBuildTrajectoryPrompt(t *testing.T) {
	messages := []agentic.Message{
		{Role: "user", Content: "List the files in the directory."},
		{
			Role: "assistant",
			ToolCalls: []agentic.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: agentic.FunctionCall{
						Name:      "bash",
						Arguments: `{"command": "ls -la"}`,
					},
				},
			},
		},
		{
			Role:    "tool",
			Content: "main.go  README.md",
			Name:    "bash",
		},
	}

	trajectory := agentic.BuildTrajectoryPrompt(messages, nil)
	if !strings.Contains(trajectory, "User: List the files") {
		t.Errorf("Trajectory missing user message")
	}
	if !strings.Contains(trajectory, "<tool>") || !strings.Contains(trajectory, "bash") {
		t.Errorf("Trajectory missing assistant tool call")
	}
	if !strings.Contains(trajectory, "Tool result (bash): main.go  README.md") {
		t.Errorf("Trajectory missing tool result")
	}
	if !strings.Contains(trajectory, "Continue the task using the tool results above.") {
		t.Errorf("Trajectory missing anchor instruction")
	}
}
