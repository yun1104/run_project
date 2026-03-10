package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func callPythonRecommendService(ctx context.Context, input llmPythonInput) (llmChatResultRaw, error) {
	endpoint := strings.TrimSpace(os.Getenv("PY_RECOMMEND_URL"))
	if endpoint == "" {
		endpoint = "http://127.0.0.1:18080/v1/recommend"
	}

	payload, err := json.Marshal(input)
	if err != nil {
		return llmChatResultRaw{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return llmChatResultRaw{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return llmChatResultRaw{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return llmChatResultRaw{}, fmt.Errorf("python recommend service status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out llmChatResultRaw
	if err := json.Unmarshal(body, &out); err != nil {
		return llmChatResultRaw{}, err
	}
	return out, nil
}

