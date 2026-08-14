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

// Multiline matching for tool call XML tags
var toolCallRegex = regexp.MustCompile(`(?s)<tool_call>(.*?)</tool_call>`)

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
		rawJSON := strings.TrimSpace(m[1])
		var parsed struct {
			Name       string `json:"name"`
			Arguments  any    `json:"arguments"`
			Parameters any    `json:"parameters"`
			Input      any    `json:"input"`
		}
		if err := json.Unmarshal([]byte(rawJSON), &parsed); err == nil && parsed.Name != "" {
			args := parsed.Arguments
			if args == nil {
				if parsed.Input != nil {
					args = parsed.Input
				} else if parsed.Parameters != nil {
					args = parsed.Parameters
				}
			}

			var argStr string
			switch a := args.(type) {
			case string:
				argStr = a
			case nil:
				argStr = "{}"
			default:
				argBytes, err := json.Marshal(a)
				if err == nil {
					argStr = string(argBytes)
				} else {
					argStr = "{}"
				}
			}

			calls = append(calls, ToolCall{
				ID:   fmt.Sprintf("call_%d", i+1),
				Type: "function",
				Function: FunctionCall{
					Name:      parsed.Name,
					Arguments: argStr,
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

