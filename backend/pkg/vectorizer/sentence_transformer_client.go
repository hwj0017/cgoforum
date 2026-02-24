package vectorizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cgoforum/config"
)

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type SentenceTransformerClient struct {
	endpoint   string
	model      string
	httpClient *http.Client
}

type embedRequest struct {
	Texts []string `json:"texts"`
	Model string   `json:"model,omitempty"`
}

type embedResponse struct {
	Vectors [][]float32 `json:"vectors"`
}

func NewSentenceTransformerClient(cfg *config.VectorizerConfig) *SentenceTransformerClient {
	timeout := 8 * time.Second
	if cfg != nil && cfg.TimeoutMS > 0 {
		timeout = time.Duration(cfg.TimeoutMS) * time.Millisecond
	}

	endpoint := "http://localhost:8001"
	model := "moka-ai/m3e-base"
	if cfg != nil {
		if strings.TrimSpace(cfg.Endpoint) != "" {
			endpoint = strings.TrimRight(cfg.Endpoint, "/")
		}
		if strings.TrimSpace(cfg.Model) != "" {
			model = strings.TrimSpace(cfg.Model)
		}
	}

	return &SentenceTransformerClient{
		endpoint: endpoint,
		model:    model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *SentenceTransformerClient) Embed(ctx context.Context, text string) ([]float32, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty text")
	}

	reqBody := embedRequest{
		Texts: []string{text},
		Model: c.model,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/embed", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("vectorizer status: %d", resp.StatusCode)
	}

	var payload embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload.Vectors) == 0 {
		return nil, fmt.Errorf("empty vectors")
	}
	return payload.Vectors[0], nil
}
