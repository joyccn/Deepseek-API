package client

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

const maxSSELineSize = 10 * 1024 * 1024 // 10MB max line buffer size for large SSE payloads

func ParseSSE(r io.Reader) <-chan StreamEvent {
	ch := make(chan StreamEvent, 200)

	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), maxSSELineSize)
		activeFragmentType := EventContent

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}

			var obj map[string]any
			if err := json.Unmarshal([]byte(payload), &obj); err != nil {
				continue
			}

			v := obj["v"]

			// Snapshot frame: {"v": {"response": {"message_id": 2, "fragments": [...]}}}
			if vMap, ok := v.(map[string]any); ok {
				if resp, ok := vMap["response"].(map[string]any); ok {
					if frags, ok := resp["fragments"].([]any); ok {
						for _, f := range frags {
							if fMap, ok := f.(map[string]any); ok {
								fType, _ := fMap["type"].(string)
								content, _ := fMap["content"].(string)
								fTypeUpper := strings.ToUpper(fType)
								if strings.Contains(fTypeUpper, "THINK") || strings.Contains(fTypeUpper, "REASON") {
									activeFragmentType = EventThinking
								} else {
									activeFragmentType = EventContent
								}
								if content != "" {
									ch <- StreamEvent{Type: activeFragmentType, Text: content}
								}
							}
						}
					}
				}
				continue
			}

			// Append frame with new fragment list: {"p": "response/fragments", "o": "APPEND", "v": [...]}
			if obj["p"] == "response/fragments" && obj["o"] == "APPEND" {
				if vList, ok := v.([]any); ok && len(vList) > 0 {
					if fMap, ok := vList[0].(map[string]any); ok {
						fType, _ := fMap["type"].(string)
						content, _ := fMap["content"].(string)
						fTypeUpper := strings.ToUpper(fType)
						if strings.Contains(fTypeUpper, "THINK") || strings.Contains(fTypeUpper, "REASON") {
							activeFragmentType = EventThinking
						} else {
							activeFragmentType = EventContent
						}
						if content != "" {
							ch <- StreamEvent{Type: activeFragmentType, Text: content}
						}
					}
				}
				continue
			}

			// Text content append: {"p": "response/fragments/0/content", "v": "..."}
			if strVal, ok := v.(string); ok && strVal != "" {
				pStr, _ := obj["p"].(string)
				fragType := activeFragmentType
				if strings.Contains(pStr, "fragments/0") {
					fragType = EventThinking
				} else if strings.Contains(pStr, "fragments/1") {
					fragType = EventContent
				}
				if pStr == "" || strings.Contains(pStr, "content") || strings.Contains(pStr, "fragments") {
					ch <- StreamEvent{Type: fragType, Text: strVal}
				}
			}
		}
	}()

	return ch
}
