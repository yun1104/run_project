package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatSendsRequestAndParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s, want /chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "qwen-turbo" {
			t.Fatalf("model = %q, want qwen-turbo", req.Model)
		}
		if len(req.Messages) != 1 || req.Messages[0].Content != "hello" {
			t.Fatalf("messages mismatch: %+v", req.Messages)
		}

		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"推荐轻食"}}]}`))
	}))
	defer server.Close()

	client := NewAIClient("test-key", server.URL)
	got, err := client.Chat("hello")
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if got != "推荐轻食" {
		t.Fatalf("Chat = %q, want 推荐轻食", got)
	}
}

func TestChatEmptyChoicesReturnsEmptyString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	client := NewAIClient("test-key", server.URL)
	got, err := client.Chat("hello")
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("Chat = %q, want empty string", got)
	}
}

func TestRecommendScoreParsesNumericResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"88.5"}}]}`))
	}))
	defer server.Close()

	client := NewAIClient("test-key", server.URL)
	score, err := client.RecommendScore("中辣", "川菜商家")
	if err != nil {
		t.Fatalf("RecommendScore returned error: %v", err)
	}
	if score != 88.5 {
		t.Fatalf("score = %v, want 88.5", score)
	}
}
