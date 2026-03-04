package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type AIClient struct {
	Endpoint   string
	Key        string
	Model      string
	Mock       bool
	HTTPClient *http.Client
}

func NewAIClient(cfg *Config) *AIClient {
	return &AIClient{
		Endpoint:   cfg.AI.Endpoint,
		Key:        cfg.AI.Key,
		Model:      cfg.AI.Model,
		Mock:       cfg.AI.Mock,
		HTTPClient: &http.Client{},
	}
}

type chatMessage struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type GenerateResult struct {
	HTML         string
	InputTokens  int
	OutputTokens int
}

const mockHTML = `<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;max-width:480px;margin:40px auto;background:#fff;border-radius:12px;box-shadow:0 2px 12px rgba(0,0,0,.08);overflow:hidden">
<div style="background:linear-gradient(135deg,#667eea,#764ba2);padding:24px;color:#fff">
<div style="display:flex;align-items:center;gap:12px;margin-bottom:16px">
<div style="width:44px;height:44px;border-radius:50%;background:rgba(255,255,255,.25);display:flex;align-items:center;justify-content:center;font-size:18px">👤</div>
<div><div style="font-weight:600;font-size:15px">示例用户</div><div style="font-size:12px;opacity:.8">2 分钟前</div></div>
</div>
<p style="font-size:15px;line-height:1.6;margin:0">这是一条示例消息 🎉<br>AI 已成功将截图转换为 HTML 页面。</p>
</div>
<div style="padding:20px">
<p style="color:#333;font-size:14px;line-height:1.7;margin:0 0 16px">截图中的文字内容、排版和配色都会被还原到这个 HTML 中。头像、Logo 等图片会用占位符替代。</p>
<div style="display:flex;gap:8px">
<span style="background:#f0f0f5;padding:6px 14px;border-radius:20px;font-size:13px;color:#555">👍 赞同 42</span>
<span style="background:#f0f0f5;padding:6px 14px;border-radius:20px;font-size:13px;color:#555">💬 评论 7</span>
<span style="background:#f0f0f5;padding:6px 14px;border-radius:20px;font-size:13px;color:#555">⭐ 收藏</span>
</div>
</div>
</div>`

func (c *AIClient) GenerateHTML(imageBase64 string, mimeType string, prompt string) (*GenerateResult, error) {
	if c.Mock {
		time.Sleep(2 * time.Second)
		return &GenerateResult{HTML: mockHTML}, nil
	}

	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, imageBase64)

	reqBody := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{
				Role: "user",
				Content: []contentPart{
					{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
					{Type: "text", Text: prompt},
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		log.Printf("unmarshal failed, status=%d body=%s", resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		log.Printf("API error response, status=%d body=%s", resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		log.Printf("no choices in response, status=%d body=%s", resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("no choices in response")
	}

	result := &GenerateResult{
		HTML: extractHTML(chatResp.Choices[0].Message.Content),
	}
	if chatResp.Usage != nil {
		result.InputTokens = chatResp.Usage.PromptTokens
		result.OutputTokens = chatResp.Usage.CompletionTokens
	}
	return result, nil
}

var htmlFenceRe = regexp.MustCompile("(?s)```(?:html)?\\s*\n(.*?)```")

func extractHTML(content string) string {
	if matches := htmlFenceRe.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "<") {
		return content
	}
	return content
}
