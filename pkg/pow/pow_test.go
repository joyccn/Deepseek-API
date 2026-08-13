package pow_test

import (
	"context"
	"testing"

	"deepseek-api/pkg/pow"
)

func TestPoWSolver(t *testing.T) {
	ctx := context.Background()
	solver, err := pow.NewSolver(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize PoW solver: %v", err)
	}
	defer solver.Close(ctx)

	challenge := map[string]any{
		"algorithm":   "DeepSeekHashV1",
		"challenge":   "7eea2d5c906a3da9dba1cf70a64776330c679f0cdc4335f249cd5f7ee71ed2b0",
		"salt":        "da9aed35ee2537201e91",
		"expire_at":   int64(1786624951519),
		"difficulty":  float64(144000.0),
		"signature":   "80f3d64f71ed4f15c420bf0bf840fcdaeb346701a9bb0a03b9490a07ee7afc95",
		"target_path": "/api/v0/chat/completion",
	}

	header, err := solver.MakeHeader(challenge)
	if err != nil {
		t.Fatalf("MakeHeader failed: %v", err)
	}
	if len(header) == 0 {
		t.Fatal("Header output is empty")
	}
	t.Logf("Generated header (base64 len=%d): %s...", len(header), header[:30])
}
