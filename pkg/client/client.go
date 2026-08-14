package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"deepseek-api/pkg/auth"
	"deepseek-api/pkg/pow"
)

const (
	BaseURL        = "https://chat.deepseek.com"
	CompletionPath = "/api/v0/chat/completion"
)

type Client struct {
	session *auth.Session
	solver  *pow.Solver
	http    *http.Client
}

func NewClient(session *auth.Session, solver *pow.Solver) *Client {
	return &Client{
		session: session,
		solver:  solver,
		http:    &http.Client{Timeout: 180 * time.Second},
	}
}

func (c *Client) CreateChatSession(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", BaseURL+"/api/v0/chat_session/create", bytes.NewBufferString("{}"))
	if err != nil {
		return "", err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create_chat_session failed with HTTP %d", resp.StatusCode)
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			BizData struct {
				ChatSession struct {
					ID string `json:"id"`
				} `json:"chat_session"`
			} `json:"biz_data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("unable to decode chat session response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("deepseek API error: %s (code %d)", result.Msg, result.Code)
	}
	return result.Data.BizData.ChatSession.ID, nil
}

func (c *Client) FetchPoWHeader(ctx context.Context) (string, error) {
	reqBody, _ := json.Marshal(map[string]string{"target_path": CompletionPath})
	req, err := http.NewRequestWithContext(ctx, "POST", BaseURL+"/api/v0/chat/create_pow_challenge", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code int `json:"code"`
		Data struct {
			BizData struct {
				Challenge map[string]any `json:"challenge"`
			} `json:"biz_data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return c.solver.MakeHeader(ctx, result.Data.BizData.Challenge)
}

func (c *Client) Stream(ctx context.Context, compReq CompletionRequest, powHeader string) (<-chan StreamEvent, error) {
	modelType := compReq.ModelType
	if modelType == "" || strings.EqualFold(modelType, "default") || strings.EqualFold(modelType, "chat") || strings.EqualFold(modelType, "deepseek-chat") {
		modelType = "DEFAULT"
	}

	bodyMap := map[string]any{
		"chat_session_id":   compReq.ChatSessionID,
		"parent_message_id": compReq.ParentMessageID,
		"prompt":            compReq.Prompt,
		"ref_file_ids":      []string{},
		"thinking_enabled":  compReq.ThinkingEnabled,
		"search_enabled":    compReq.SearchEnabled,
		"action":            nil,
		"preempt":           false,
		"model_type":        modelType,
	}

	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal completion payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", BaseURL+CompletionPath, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	req.Header.Set("x-ds-pow-response", powHeader)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP completion returned status %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan StreamEvent, 200)
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		parsed := ParseSSE(resp.Body)
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-parsed:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case ch <- ev:
				}
			}
		}
	}()

	return ch, nil
}

func (c *Client) AcquireAccessToken(ctx context.Context) (string, error) {
	token := c.session.Token
	if token == "" {
		return "", fmt.Errorf("no token available in session")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", BaseURL+"/api/v0/users/current", nil)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("users/current request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("users/current HTTP %d", resp.StatusCode)
	}

	var res struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			BizData struct {
				Token string `json:"token"`
			} `json:"biz_data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("decode users/current response failed: %w", err)
	}
	if res.Code != 0 {
		return "", fmt.Errorf("deepseek rejected token: %s (code %d)", res.Msg, res.Code)
	}

	accessToken := res.Data.BizData.Token
	if accessToken == "" {
		return "", fmt.Errorf("no access token returned in biz_data")
	}

	c.session.AccessToken = accessToken
	c.session.AccessTokenExpiresAt = time.Now().Unix() + 3600
	return accessToken, nil
}

func (c *Client) DeleteChatSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]string{"chat_session_id": sessionID})
	req, err := http.NewRequestWithContext(ctx, "POST", BaseURL+"/api/v0/chat_session/delete", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	c.setHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	effectiveToken := c.session.GetEffectiveToken()
	req.Header.Set("Authorization", "Bearer "+effectiveToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.session.UserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", BaseURL)
	req.Header.Set("Referer", BaseURL+"/")
	req.Header.Set("X-Client-Version", "2.0.0")
	req.Header.Set("X-Client-Platform", "web")
	req.Header.Set("X-Client-Locale", "en_US")
	req.Header.Set("X-Client-Bundle-Id", "com.deepseek.chat")

	for k, v := range c.session.Cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
}
