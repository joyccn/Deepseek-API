package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
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

func (c *Client) UploadFile(ctx context.Context, fileName string, fileBytes []byte) (string, error) {
	powHeader, _ := c.FetchPoWHeaderForPath(ctx, "/api/v0/file/upload")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(fileBytes); err != nil {
		return "", err
	}
	_ = writer.WriteField("target", "chat")
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", BaseURL+"/api/v0/file/upload", &body)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if powHeader != "" {
		req.Header.Set("x-ds-pow-response", powHeader)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyRespBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("deepseek file upload returned HTTP %d: %s", resp.StatusCode, string(bodyRespBytes))
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			BizData struct {
				ID string `json:"id"`
			} `json:"biz_data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyRespBytes, &result); err != nil {
		return "", fmt.Errorf("unable to decode file upload response (body=%s): %w", string(bodyRespBytes), err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("deepseek file upload error: %s (code %d)", result.Msg, result.Code)
	}
	return result.Data.BizData.ID, nil
}

func (c *Client) FetchPoWHeader(ctx context.Context) (string, error) {
	return c.FetchPoWHeaderForPath(ctx, CompletionPath)
}

func (c *Client) FetchPoWHeaderForPath(ctx context.Context, targetPath string) (string, error) {
	reqBody, _ := json.Marshal(map[string]string{"target_path": targetPath})
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
	return c.solver.MakeHeader(result.Data.BizData.Challenge)
}

func (c *Client) Stream(ctx context.Context, compReq CompletionRequest, powHeader string) (<-chan StreamEvent, error) {
	refFileIDs := []string{}
	if len(compReq.RefFileIDs) > 0 {
		refFileIDs = compReq.RefFileIDs
	}

	modelType := compReq.ModelType
	thinkingEnabled := compReq.ThinkingEnabled

	if len(refFileIDs) > 0 {
		modelType = "vision"
		thinkingEnabled = false
	} else if modelType == "" || modelType == "default" {
		modelType = "DEFAULT"
	}

	bodyMap := map[string]any{
		"chat_session_id":   compReq.ChatSessionID,
		"parent_message_id": compReq.ParentMessageID,
		"prompt":            compReq.Prompt,
		"ref_file_ids":      refFileIDs,
		"thinking_enabled":  thinkingEnabled,
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
		for ev := range parsed {
			ch <- ev
		}
	}()

	return ch, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.session.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.session.UserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", BaseURL)
	req.Header.Set("Referer", BaseURL+"/")
	req.Header.Set("x-app-version", "2.0.0")
	req.Header.Set("x-client-version", "2.0.0")
	req.Header.Set("x-client-platform", "web")
	req.Header.Set("x-client-locale", "en_US")
	req.Header.Set("x-client-bundle-id", "com.deepseek.chat")

	for k, v := range c.session.Cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
}
