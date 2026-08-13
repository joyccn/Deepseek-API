package auth_test

import (
	"path/filepath"
	"testing"
	"time"

	"deepseek-api/pkg/auth"
)

func TestSessionStorage(t *testing.T) {
	dir := t.TempDir()
	sessPath := filepath.Join(dir, "session.json")

	sess := &auth.Session{
		Token: "test_token_123456",
		Cookies: map[string]string{
			"ds_session_id": "test_session_id_999",
		},
		UserAgent:  "Mozilla/5.0 TestBrowser",
		CapturedAt: float64(time.Now().Unix()),
	}

	if err := sess.Save(sessPath); err != nil {
		t.Fatalf("Save session failed: %v", err)
	}

	loaded, err := auth.LoadSession(sessPath)
	if err != nil {
		t.Fatalf("Load session failed: %v", err)
	}
	if loaded.Token != sess.Token {
		t.Errorf("Expected token %s, got %s", sess.Token, loaded.Token)
	}
	if loaded.Cookies["ds_session_id"] != "test_session_id_999" {
		t.Errorf("Expected cookie ds_session_id test_session_id_999, got %s", loaded.Cookies["ds_session_id"])
	}
	if !loaded.IsValid() {
		t.Errorf("Expected loaded session to be valid")
	}
}
