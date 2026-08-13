package agentic

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

var toolCallRegex = regexp.MustCompile(`<tool_call>(.*?)</tool_call>`)

func ParseToolCalls(content string) ([]ToolCall, string) {
	matches := toolCallRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, content
	}

	var calls []ToolCall
	for i, m := range matches {
		if len(m) < 2 {
			continue
		}
		var parsed struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(m[1]), &parsed); err == nil {
			argBytes, _ := json.Marshal(parsed.Arguments)
			calls = append(calls, ToolCall{
				ID:   fmt.Sprintf("call_%d", i+1),
				Type: "function",
				Function: FunctionCall{
					Name:      parsed.Name,
					Arguments: string(argBytes),
				},
			})
		}
	}

	cleanText := toolCallRegex.ReplaceAllString(content, "")
	return calls, strings.TrimSpace(cleanText)
}

func InjectToolsIntoPrompt(prompt string, tools []any) string {
	if len(tools) == 0 {
		return prompt
	}

	toolBytes, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return prompt
	}

	instructions := fmt.Sprintf(
		"\n\n[AVAILABLE TOOLS]\n%s\n\n"+
			"[TOOL CALLING INSTRUCTION]\n"+
			"To call a tool, output a JSON object wrapped inside <tool_call>...</tool_call> tags format:\n"+
			"<tool_call>{\"name\": \"function_name\", \"arguments\": {\"arg\": \"val\"}}</tool_call>\n",
		string(toolBytes),
	)

	return prompt + instructions
}
