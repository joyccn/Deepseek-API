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

type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// Regex patterns for tool call variations
var (
	toolCallTagRegex = regexp.MustCompile(`(?s)<tool_call(?:\s+[^>]*)?>(.*?)</tool_call>`)
	toolTagRegex     = regexp.MustCompile(`(?s)<tool(?::([a-zA-Z0-9_.-]+))?(?:\s+([^>]*))?>(.*?)</tool>`)
	toolAttrRegex    = regexp.MustCompile(`(?:name|id)=["']([^"']+)["']`)
	parameterRegex   = regexp.MustCompile(`(?s)<parameter\s+name=["']([^"']+)["'](?:\s+content=["']([^"']*)["']\s*/?>|>(.*?)</parameter>)`)
)

func ParseToolCalls(content string) ([]ToolCall, string) {
	var calls []ToolCall
	cleanText := content

	// Helper to extract JSON arguments or fallback map
	normalizeArgs := func(raw any) string {
		if raw == nil {
			return "{}"
		}
		if s, ok := raw.(string); ok {
			return s
		}
		b, err := json.Marshal(raw)
		if err != nil {
			return "{}"
		}
		return string(b)
	}

	// 1. Match <tool_call>...</tool_call>
	toolCallMatches := toolCallTagRegex.FindAllStringSubmatch(content, -1)
	for i, m := range toolCallMatches {
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
			calls = append(calls, ToolCall{
				ID:   fmt.Sprintf("call_%d", len(calls)+1+i),
				Type: "function",
				Function: FunctionCall{
					Name:      parsed.Name,
					Arguments: normalizeArgs(args),
				},
			})
		}
	}
	cleanText = toolCallTagRegex.ReplaceAllString(cleanText, "")

	// 2. Match <tool:name>...</tool> or <tool name="...">...</tool> or <tool>{json}</tool>
	toolMatches := toolTagRegex.FindAllStringSubmatch(cleanText, -1)
	for i, m := range toolMatches {
		if len(m) < 4 {
			continue
		}
		suffixName := strings.TrimSpace(m[1])
		attrs := m[2]
		inner := strings.TrimSpace(m[3])

		toolName := suffixName
		if toolName == "" && attrs != "" {
			if attrMatch := toolAttrRegex.FindStringSubmatch(attrs); len(attrMatch) > 1 {
				toolName = attrMatch[1]
			}
		}

		// Try parsing inner as XML parameter tags
		paramMatches := parameterRegex.FindAllStringSubmatch(inner, -1)
		if len(paramMatches) > 0 {
			params := make(map[string]any)
			for _, pm := range paramMatches {
				pName := pm[1]
				pVal := pm[2]
				if pVal == "" && len(pm) > 3 {
					pVal = pm[3]
				}
				params[pName] = strings.TrimSpace(pVal)
			}
			if toolName != "" {
				calls = append(calls, ToolCall{
					ID:   fmt.Sprintf("call_%d", len(calls)+1+i),
					Type: "function",
					Function: FunctionCall{
						Name:      toolName,
						Arguments: normalizeArgs(params),
					},
				})
				continue
			}
		}

		// Try parsing inner as JSON object
		var parsed map[string]any
		if err := json.Unmarshal([]byte(inner), &parsed); err == nil {
			if toolName == "" {
				if n, ok := parsed["name"].(string); ok && n != "" {
					toolName = n
				} else if n, ok := parsed["type"].(string); ok && n != "" {
					toolName = n
				}
			}

			args := parsed["arguments"]
			if args == nil {
				if parsed["input"] != nil {
					args = parsed["input"]
				} else if parsed["parameters"] != nil {
					args = parsed["parameters"]
				} else if toolName != "" && suffixName != "" {
					// <tool:bash>{"command": "..."} -> entire json is arguments
					args = parsed
				} else {
					// Remaining fields are arguments
					copied := make(map[string]any)
					for k, v := range parsed {
						if k != "name" && k != "type" && k != "id" && k != "_nonce" {
							copied[k] = v
						}
					}
					args = copied
				}
			}

			if toolName != "" {
				calls = append(calls, ToolCall{
					ID:   fmt.Sprintf("call_%d", len(calls)+1+i),
					Type: "function",
					Function: FunctionCall{
						Name:      toolName,
						Arguments: normalizeArgs(args),
					},
				})
			}
		}
	}
	cleanText = toolTagRegex.ReplaceAllString(cleanText, "")

	if len(calls) == 0 {
		return nil, content
	}
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
			"To call a tool, output a JSON block wrapped in <tool_call>...</tool_call> or <tool>...</tool> tags:\n"+
			"<tool>{\"name\": \"tool_name\", \"arguments\": {\"arg\": \"val\"}}</tool>\n"+
			"Rules:\n"+
			"- Output ONLY the <tool>...</tool> block when calling a tool.\n"+
			"- If no tool is needed, respond with standard answer text.\n",
		string(toolBytes),
	)

	return prompt + instructions
}

// BuildTrajectoryPrompt stitches multi-turn agent turns into a coherent transcript
func BuildTrajectoryPrompt(messages []Message, tools []any) string {
	var toolPrompt string
	if len(tools) > 0 {
		toolPrompt = InjectToolsIntoPrompt("", tools)
	}

	var sb strings.Builder
	if toolPrompt != "" {
		sb.WriteString(toolPrompt)
		sb.WriteString("\n\n")
	}

	sawToolActivity := false
	for _, m := range messages {
		text := extractMessageText(m.Content)
		switch m.Role {
		case "system":
			if text != "" {
				sb.WriteString(fmt.Sprintf("[SYSTEM]\n%s\n\n", text))
			}
		case "user":
			if text != "" {
				sb.WriteString(fmt.Sprintf("User: %s\n\n", text))
			}
		case "assistant":
			if len(m.ToolCalls) > 0 {
				sawToolActivity = true
				var parts []string
				if text != "" {
					parts = append(parts, text)
				}
				for _, tc := range m.ToolCalls {
					parts = append(parts, fmt.Sprintf("<tool>{\"name\": %q, \"arguments\": %s}</tool>", tc.Function.Name, tc.Function.Arguments))
				}
				sb.WriteString(fmt.Sprintf("Assistant: %s\n\n", strings.Join(parts, "\n")))
			} else if text != "" {
				sb.WriteString(fmt.Sprintf("Assistant: %s\n\n", text))
			}
		case "tool":
			sawToolActivity = true
			name := m.Name
			if name == "" {
				name = "tool"
			}
			sb.WriteString(fmt.Sprintf("Tool result (%s): %s\n\n", name, text))
		}
	}

	if sawToolActivity {
		sb.WriteString("Continue the task using the tool results above. Do NOT repeat tool calls that already succeeded; perform the next step or give the final answer.\n")
	}

	return strings.TrimSpace(sb.String())
}

func extractMessageText(content any) string {
	if content == nil {
		return ""
	}
	if s, ok := content.(string); ok {
		return strings.TrimSpace(s)
	}
	if list, ok := content.([]any); ok {
		var parts []string
		for _, item := range list {
			if m, ok := item.(map[string]any); ok {
				if t, ok := m["text"].(string); ok && t != "" {
					parts = append(parts, t)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return fmt.Sprintf("%v", content)
}


