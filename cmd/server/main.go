package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"deepseek-api/pkg/auth"
	"deepseek-api/pkg/client"
	"deepseek-api/pkg/pow"
	"deepseek-api/pkg/server"
)

func main() {
	ctx := context.Background()
	slog.Info("Starting DeepSeek API Native Golang Relay Server...")

	sessionPath := os.Getenv("SESSION_FILE")
	if sessionPath == "" {
		sessionPath = filepath.Join("session", "session.json")
	}

	sess, err := auth.LoadSession(sessionPath)
	if err != nil {
		slog.Error("Failed to load session file", "path", sessionPath, "error", err)
		slog.Info("Please run python -m deepseek.auth first or ensure session/session.json exists")
		os.Exit(1)
	}

	solver, err := pow.NewSolver(ctx)
	if err != nil {
		slog.Error("Failed to initialize WASM PoW solver", "error", err)
		os.Exit(1)
	}
	defer solver.Close(ctx)

	cli := client.NewClient(sess, solver)
	srv := server.NewServer(cli)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}

	addr := host + ":" + port
	slog.Info("DeepSeek OpenAI-compatible server running", "address", "http://"+addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("Server shutdown with error", "error", err)
	}
}
