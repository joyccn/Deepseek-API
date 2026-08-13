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

	// Create a chat session
	sessionID, err := cli.CreateChatSession(ctx)
	if err != nil {
		log.Fatalf("Failed to create chat session: %v", err)
	}
	fmt.Printf("Created Chat Session: %s\n", sessionID)

	// Fetch PoW Header
	powHeader, err := cli.FetchPoWHeader(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch PoW header: %v", err)
	}

	req := client.CompletionRequest{
		Prompt:          "Say hello in 1 short sentence.",
		ChatSessionID:   sessionID,
		ModelType:       "default",
		ThinkingEnabled: false,
		SearchEnabled:   false,
	}

	events, err := cli.Stream(ctx, req, powHeader)
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	fmt.Print("Response: ")
	for ev := range events {
		if ev.Type == client.EventContent {
			fmt.Print(ev.Text)
		}
	}
	fmt.Println()
}
