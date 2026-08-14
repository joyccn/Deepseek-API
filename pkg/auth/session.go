package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Session struct {
	Token                string            `json:"token"`
	UserToken            string            `json:"user_token,omitempty"`
	AccessToken          string            `json:"access_token,omitempty"`
	AccessTokenExpiresAt int64             `json:"access_token_expires_at,omitempty"`
	Cookies              map[string]string `json:"cookies"`
	UserAgent            string            `json:"user_agent"`
	CapturedAt           float64           `json:"captured_at"`
}

// ExtractUserToken unmarshals JSON wrapped token {"value":"..."} or returns raw string
func ExtractUserToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "{") && strings.HasSuffix(raw, "}") {
		var parsed struct {
			Value string `json:"value"`
			Token string `json:"token"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
			if parsed.Value != "" {
				return parsed.Value
			}
			if parsed.Token != "" {
				return parsed.Token
			}
		}
	}
	return raw
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
	s.Token = ExtractUserToken(s.Token)
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

func (s *Session) GetEffectiveToken() string {
	now := time.Now().Unix()
	if s.AccessToken != "" && s.AccessTokenExpiresAt > now+30 {
		return s.AccessToken
	}
	return s.Token
}

func (s *Session) IsValid() bool {
	return s.GetEffectiveToken() != ""
}

