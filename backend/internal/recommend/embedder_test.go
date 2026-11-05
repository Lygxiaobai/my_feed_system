package recommend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"my_feed_system/internal/config"
)

func TestHTTPEmbedderParsesOpenAIShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth=%q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"embedding": []float32{3, 4}},
			},
		})
	}))
	defer srv.Close()

	e := NewHTTPEmbedder(config.EmbeddingConfig{
		APIURL: srv.URL,
		APIKey: "test-key",
		Model:  "unit-test",
	})
	vec, err := e.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 2 {
		t.Fatalf("dim=%d", len(vec))
	}
	// 3-4-5 三角形，归一化后约 0.6 / 0.8
	if vec[0] < 0.5 || vec[1] < 0.7 {
		t.Fatalf("normalized %v", vec)
	}
}

func TestEmbeddingEndpointAcceptsBaseOrFullURL(t *testing.T) {
	if got := embeddingEndpoint("https://router.example/v1"); got != "https://router.example/v1/embeddings" {
		t.Fatal(got)
	}
	if got := embeddingEndpoint("https://router.example/v1/embeddings"); got != "https://router.example/v1/embeddings" {
		t.Fatal(got)
	}
}

func TestHTTPEmbedderDisabled(t *testing.T) {
	e := NewHTTPEmbedder(config.EmbeddingConfig{})
	if e.Enabled() {
		t.Fatal("empty config should disable embedder")
	}
}
