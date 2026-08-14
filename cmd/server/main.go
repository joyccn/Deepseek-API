package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"deepseek-api/pkg/auth"
	"deepseek-api/pkg/client"
	"deepseek-api/pkg/pow"
	"deepseek-api/pkg/server"
)

func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

func main() {
	loadDotEnv(".env")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	slog.Info("Starting DeepSeek API Native Golang Relay Server...")

	sessionPath := os.Getenv("SESSION_FILE")
	if sessionPath == "" {
		sessionPath = filepath.Join("session", "session.json")
	}

	sess, err := auth.LoadSession(sessionPath)
	if err != nil {
		slog.Error("Failed to load session file", "path", sessionPath, "error", err)
		slog.Info("Please ensure session/session.json exists with valid token and cookies")
		os.Exit(1)
	}

	if sess.Token == "" {
		slog.Warn("Session token in session/session.json is empty. Please ensure valid bearer token is set.")
	}

	solver, err := pow.NewSolver(ctx)
	if err != nil {
		slog.Error("Failed to initialize WASM PoW solver", "error", err)
		os.Exit(1)
	}
	defer solver.Close(context.Background())

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
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		slog.Info("DeepSeek OpenAI-compatible server running", "address", "http://"+addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	}
	slog.Info("Server stopped successfully")
}

