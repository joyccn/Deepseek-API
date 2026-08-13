package client_test

import (
	"strings"
	"testing"

	"deepseek-api/pkg/client"
)

func TestParseSSEStream(t *testing.T) {
	rawSSE := `data: {"v":{"response":{"message_id":2,"fragments":[{"id":2,"type":"THINK","content":"Thinking hard..."}]}}}
data: {"p":"response/fragments/-1/content","o":"APPEND","v":" step 1"}
data: {"p":"response/fragments","o":"APPEND","v":[{"id":3,"type":"RESPONSE","content":"Hello world"}]}
data: {"p":"response/fragments/-1/content","v":"!"}`

	events := client.ParseSSE(strings.NewReader(rawSSE))
	var thinkParts, contentParts []string

	for ev := range events {
		if ev.Type == client.EventThinking {
			thinkParts = append(thinkParts, ev.Text)
		} else if ev.Type == client.EventContent {
			contentParts = append(contentParts, ev.Text)
		}
	}

	thinkStr := strings.Join(thinkParts, "")
	contentStr := strings.Join(contentParts, "")

	if thinkStr != "Thinking hard... step 1" {
		t.Errorf("Unexpected thinking output: %q, expected 'Thinking hard... step 1'", thinkStr)
	}
	if contentStr != "Hello world!" {
		t.Errorf("Unexpected content output: %q, expected 'Hello world!'", contentStr)
	}
}
