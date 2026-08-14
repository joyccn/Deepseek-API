package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const maxSSELineSize = 10 * 1024 * 1024 // 10MB max line buffer size for large SSE payloads

var (
	citationTagRegex = regexp.MustCompile(`\[citation:(\d+)\]`)
	searchPrefixRegex = regexp.MustCompile(`(?i)^(SEARCH|WEB_SEARCH|SEARCHING)\s*`)
)

type SearchCitation struct {
	CiteIndex int    `json:"cite_index"`
	Title     string `json:"title"`
	URL       string `json:"url"`
}

func ParseSSE(r io.Reader) <-chan StreamEvent {
	ch := make(chan StreamEvent, 200)

	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), maxSSELineSize)

		fragmentTypes := make(map[int]StreamEventType)
		activeFragmentType := EventContent
		var searchCitations []SearchCitation

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

			pStr, _ := obj["p"].(string)
			v := obj["v"]

			// Parse Search Results: {"p": "response/search_results", "v": [...]}
			if pStr == "response/search_results" {
				if vList, ok := v.([]any); ok {
					for _, item := range vList {
						if itemMap, ok := item.(map[string]any); ok {
							var cit SearchCitation
							if idx, ok := itemMap["cite_index"].(float64); ok {
								cit.CiteIndex = int(idx)
							} else if idx, ok := itemMap["cite_index"].(int); ok {
								cit.CiteIndex = idx
							}
							if title, ok := itemMap["title"].(string); ok {
								cit.Title = title
							}
							if url, ok := itemMap["url"].(string); ok {
								cit.URL = url
							}
							if cit.CiteIndex > 0 && cit.URL != "" {
								searchCitations = append(searchCitations, cit)
							}
						}
					}
				}
				continue
			}

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
								if content != "" {
									cleanContent := cleanStreamText(content)
									if cleanContent != "" {
										ch <- StreamEvent{Type: evType, Text: cleanContent}
									}
								}
							}
						}
					}
				}
				continue
			}

			// 2. Append frame: {"p": "response/fragments", "o": "APPEND", "v": [...]}
			if pStr == "response/fragments" && obj["o"] == "APPEND" {
				if vList, ok := v.([]any); ok {
					for _, f := range vList {
						if fMap, ok := f.(map[string]any); ok {
							fType, _ := fMap["type"].(string)
							content, _ := fMap["content"].(string)
							evType := resolveFragmentType(fType)
							idx := len(fragmentTypes)
							fragmentTypes[idx] = evType
							activeFragmentType = evType
							if content != "" {
								cleanContent := cleanStreamText(content)
								if cleanContent != "" {
									ch <- StreamEvent{Type: evType, Text: cleanContent}
								}
							}
						}
					}
				}
				continue
			}

			// 3. Content append/patch: {"p": "response/fragments/<idx>/content", "v": "..."}
			if strVal, ok := v.(string); ok && strVal != "" {
				if strings.HasPrefix(pStr, "response/fragments/") && strings.HasSuffix(pStr, "/content") {
					idxStr := strings.TrimPrefix(pStr, "response/fragments/")
					idxStr = strings.TrimSuffix(idxStr, "/content")

					evType := activeFragmentType
					if idx, err := strconv.Atoi(idxStr); err == nil && idx >= 0 {
						if t, found := fragmentTypes[idx]; found {
							evType = t
						}
					}

					cleanText := cleanStreamText(strVal)
					if cleanText != "" {
						ch <- StreamEvent{Type: evType, Text: cleanText}
					}
				} else if pStr == "response/content" || pStr == "" {
					cleanText := cleanStreamText(strVal)
					if cleanText != "" {
						ch <- StreamEvent{Type: activeFragmentType, Text: cleanText}
					}
				}
			}
		}

		// Append citations footer if search citations were collected
		if len(searchCitations) > 0 {
			sort.Slice(searchCitations, func(i, j int) bool {
				return searchCitations[i].CiteIndex < searchCitations[j].CiteIndex
			})
			var sb strings.Builder
			sb.WriteString("\n\n")
			seen := make(map[int]bool)
			for _, c := range searchCitations {
				if !seen[c.CiteIndex] {
					seen[c.CiteIndex] = true
					title := c.Title
					if title == "" {
						title = c.URL
					}
					sb.WriteString(fmt.Sprintf("[%d]: [%s](%s)\n", c.CiteIndex, title, c.URL))
				}
			}
			ch <- StreamEvent{Type: EventContent, Text: sb.String()}
		}
	}()

	return ch
}

func cleanStreamText(text string) string {
	cleaned := strings.ReplaceAll(text, "FINISHED", "")
	cleaned = searchPrefixRegex.ReplaceAllString(cleaned, "")
	cleaned = citationTagRegex.ReplaceAllString(cleaned, "[$1]")
	return cleaned
}

func resolveFragmentType(fType string) StreamEventType {
	fTypeUpper := strings.ToUpper(fType)
	if strings.Contains(fTypeUpper, "THINK") || strings.Contains(fTypeUpper, "REASON") {
		return EventThinking
	}
	return EventContent
}
