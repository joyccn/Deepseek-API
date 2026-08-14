package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"deepseek-api/pkg/auth"
	"deepseek-api/pkg/client"
	"deepseek-api/pkg/server"
)

func setupTestServer(t *testing.T) *http.ServeMux {
	t.Helper()
	sess := &auth.Session{
		Token:     "mock_token",
		Cookies:   map[string]string{"ds_session_id": "mock_id"},
		UserAgent: "Mozilla/5.0",
	}
	cli := client.NewClient(sess, nil)
	srv := server.NewServer(cli)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	return mux
}

func TestHealthzEndpoint(t *testing.T) {
	mux := setupTestServer(t)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Errorf("Expected status: ok, got %s", w.Body.String())
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected CORS Access-Control-Allow-Origin: *")
	}
}

func TestListModelsEndpoint(t *testing.T) {
	mux := setupTestServer(t)

	req := httptest.NewRequest("GET", "/v1/models", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200, got %d", w.Code)
	}

	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}
	if resp.Object != "list" {
		t.Errorf("Expected object list, got %s", resp.Object)
	}
	if len(resp.Data) < 2 {
		t.Errorf("Expected at least 2 models, got %d", len(resp.Data))
	}
}

func TestCORSPreflightOptions(t *testing.T) {
	mux := setupTestServer(t)

	req := httptest.NewRequest("OPTIONS", "/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("Expected HTTP 204 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Missing Access-Control-Allow-Origin header")
	}
	if !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "POST") {
		t.Errorf("Expected POST in Access-Control-Allow-Methods")
	}
}

func TestEmptyMessagesBadRequest(t *testing.T) {
	mux := setupTestServer(t)

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"deepseek-chat","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected HTTP 400 for empty messages, got %d", w.Code)
	}
}
