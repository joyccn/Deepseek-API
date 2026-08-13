package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ImageGenerationRequest struct {
	Prompt         string `json:"prompt"`
	Model          string `json:"model,omitempty"`
	N              int    `json:"n,omitempty"`
	Quality        string `json:"quality,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"` // "url" or "b64_json"
	Size           string `json:"size,omitempty"`
	Style          string `json:"style,omitempty"`
}

type ImageData struct {
	URL     string `json:"url,omitempty"`
	B64JSON string `json:"b64_json,omitempty"`
}

type ImageGenerationResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
}

func (s *Server) handleImageGenerations(w http.ResponseWriter, r *http.Request) {
	var req ImageGenerationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":{"message":"Invalid JSON payload: %v"}}`, err), http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		http.Error(w, `{"error":{"message":"prompt is required"}}`, http.StatusBadRequest)
		return
	}

	created := time.Now().Unix()
	resp := ImageGenerationResponse{
		Created: created,
		Data: []ImageData{
			{
				URL: "https://via.placeholder.com/1024x1024.png?text=DeepSeek+Image+Bridge",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
