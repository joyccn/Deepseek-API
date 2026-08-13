package client

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

func ParseSSE(r io.Reader) <-chan StreamEvent {
	ch := make(chan StreamEvent, 200)

	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
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
								if fType == "THINK" {
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
						if fType == "THINK" {
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

			// Text content append: {"p": "response/fragments/-1/content", "v": "..."} or {"v": "..."}
			if strVal, ok := v.(string); ok {
				pStr, _ := obj["p"].(string)
				if pStr == "" || strings.HasSuffix(pStr, "content") {
					ch <- StreamEvent{Type: activeFragmentType, Text: strVal}
				}
			}
		}
	}()

	return ch
}
