package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Session struct {
	Token      string            `json:"token"`
	Cookies    map[string]string `json:"cookies"`
	UserAgent  string            `json:"user_agent"`
	CapturedAt float64           `json:"captured_at"`
}

func LoadSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read session file: %w", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unable to parse session json: %w", err)
	}
	return &s, nil
}

func (s *Session) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("unable to create session directory: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal session json: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

func (s *Session) IsValid() bool {
	if s.Token == "" {
		return false
	}
	// Session valid for 6 hours (21600 seconds)
	maxAge := 6.0 * 3600.0
	age := float64(time.Now().Unix()) - s.CapturedAt
	return age >= 0 && age < maxAge
}
