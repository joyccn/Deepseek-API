package client

import (
	"bufio"
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

const maxSSELineSize = 10 * 1024 * 1024 // 10MB max line buffer size for large SSE payloads

func ParseSSE(r io.Reader) <-chan StreamEvent {
	ch := make(chan StreamEvent, 200)

	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), maxSSELineSize)

		fragmentTypes := make(map[int]StreamEventType)
		activeFragmentType := EventContent
		latestFragmentIndex := 0

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

			// 1. Snapshot frame: {"v": {"response": {"message_id": 2, "fragments": [...]}}}
			if vMap, ok := v.(map[string]any); ok {
				if resp, ok := vMap["response"].(map[string]any); ok {
					if frags, ok := resp["fragments"].([]any); ok {
						for idx, f := range frags {
							if fMap, ok := f.(map[string]any); ok {
								fType, _ := fMap["type"].(string)
								content, _ := fMap["content"].(string)
								evType := resolveFragmentType(fType)
								fragmentTypes[idx] = evType
								activeFragmentType = evType
								latestFragmentIndex = idx
								if content != "" {
									ch <- StreamEvent{Type: evType, Text: content}
								}
							}
						}
					}
				}
				continue
			}

			// 2. Append frame: {"p": "response/fragments", "o": "APPEND", "v": [...]}
			if obj["p"] == "response/fragments" && obj["o"] == "APPEND" {
				if vList, ok := v.([]any); ok {
					for _, f := range vList {
						if fMap, ok := f.(map[string]any); ok {
							fType, _ := fMap["type"].(string)
							content, _ := fMap["content"].(string)
							evType := resolveFragmentType(fType)
							latestFragmentIndex = len(fragmentTypes)
							fragmentTypes[latestFragmentIndex] = evType
							activeFragmentType = evType
							if content != "" {
								ch <- StreamEvent{Type: evType, Text: content}
							}
						}
					}
				}
				continue
			}

			// 3. Content append/patch: {"p": "response/fragments/<idx>/content", "v": "..."}
			pStr, _ := obj["p"].(string)
			if strVal, ok := v.(string); ok && strVal != "" {
				if strings.HasPrefix(pStr, "response/fragments/") && strings.HasSuffix(pStr, "/content") {
					idxStr := strings.TrimPrefix(pStr, "response/fragments/")
					idxStr = strings.TrimSuffix(idxStr, "/content")

					evType := activeFragmentType
					if idxStr == "-1" {
						evType = activeFragmentType
					} else if idx, err := strconv.Atoi(idxStr); err == nil {
						if t, found := fragmentTypes[idx]; found {
							evType = t
						}
					}

					ch <- StreamEvent{Type: evType, Text: strVal}
				} else if pStr == "response/content" || pStr == "" {
					ch <- StreamEvent{Type: activeFragmentType, Text: strVal}
				}
			}
		}
	}()

	return ch
}

func resolveFragmentType(fType string) StreamEventType {
	fTypeUpper := strings.ToUpper(fType)
	if strings.Contains(fTypeUpper, "THINK") || strings.Contains(fTypeUpper, "REASON") {
		return EventThinking
	}
	return EventContent
}
