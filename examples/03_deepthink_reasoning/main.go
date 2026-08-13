package main

import (
	"context"
	"fmt"
	"log"

	"deepseek-api/pkg/auth"
	"deepseek-api/pkg/client"
	"deepseek-api/pkg/pow"
)

func main() {
	ctx := context.Background()

	sess, err := auth.LoadSession("session/session.json")
	if err != nil {
		log.Fatalf("Failed to load session: %v", err)
	}

	solver, err := pow.NewSolver(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize PoW solver: %v", err)
	}
	defer solver.Close(ctx)

	cli := client.NewClient(sess, solver)

	sessionID, err := cli.CreateChatSession(ctx)
	if err != nil {
		log.Fatalf("Failed to create chat session: %v", err)
	}

	powHeader, err := cli.FetchPoWHeader(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch PoW header: %v", err)
	}

	req := client.CompletionRequest{
		Prompt:          "Which is bigger: 9.11 or 9.9?",
		ChatSessionID:   sessionID,
		ModelType:       "default",
		ThinkingEnabled: true, // Enable DeepThink R1 Reasoning
		SearchEnabled:   false,
	}

	events, err := cli.Stream(ctx, req, powHeader)
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	fmt.Println("=== THINKING PROCESS ===")
	for ev := range events {
		if ev.Type == client.EventThinking {
			fmt.Print(ev.Text)
		} else if ev.Type == client.EventContent {
			// Switched to answer
			fmt.Print(ev.Text)
		}
	}
	fmt.Println()
}
