package agentic

import (
	"regexp"
	"strings"
)

var thinkTagRegex = regexp.MustCompile(`(?s)<think>(.*?)</think>`)

func ExtractThinkingContent(text string) (string, string) {
	matches := thinkTagRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return "", text
	}
	var thinkParts []string
	for _, m := range matches {
		if len(m) >= 2 {
			thinkParts = append(thinkParts, strings.TrimSpace(m[1]))
		}
	}
	cleanText := thinkTagRegex.ReplaceAllString(text, "")
	return strings.Join(thinkParts, "\n"), strings.TrimSpace(cleanText)
}
