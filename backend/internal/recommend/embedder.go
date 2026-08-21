package recommend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"my_feed_system/internal/config"
)

// Embedder 把一段文本变成固定维度向量。HTTP 实现可替换，混排不依赖供应商。
type Embedder interface {
	Enabled() bool
	Model() string
	Embed(ctx context.Context, text string) ([]float32, error)
}

type HTTPEmbedder struct {
	cfg    config.EmbeddingConfig
	client *http.Client
}

func NewHTTPEmbedder(cfg config.EmbeddingConfig) *HTTPEmbedder {
	cfg.ApplyDefaults()
	return &HTTPEmbedder{
		cfg: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		},
	}
}

func (e *HTTPEmbedder) Enabled() bool {
	return e != nil && e.cfg.Enabled()
}

func (e *HTTPEmbedder) Model() string {
	if e == nil {
		return ""
	}
	return e.cfg.Model
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (e *HTTPEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if !e.Enabled() {
		return nil, fmt.Errorf("embedding 未配置")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("embedding 输入为空")
	}

	body, err := json.Marshal(embedRequest{Model: e.cfg.Model, Input: text})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, embeddingEndpoint(e.cfg.APIURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding HTTP %d", resp.StatusCode)
	}

	var parsed embedResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embedding 响应为空")
	}
	return normalize(parsed.Data[0].Embedding), nil
}

// embeddingEndpoint 同时接受 OpenAI 基址（.../v1）和完整路径（.../v1/embeddings）。
func embeddingEndpoint(raw string) string {
	u := strings.TrimRight(strings.TrimSpace(raw), "/")
	if u == "" || strings.HasSuffix(u, "/embeddings") {
		return u
	}
	return u + "/embeddings"
}

func EmbedText(title, description string) string {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if description == "" {
		return title
	}
	if title == "" {
		return description
	}
	return title + "\n\n" + description
}
