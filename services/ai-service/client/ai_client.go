package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type AIClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewAIClient(apiKey, baseURL string) *AIClient {
	return &AIClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

func (c *AIClient) Chat(prompt string) (string, error) {
	req := ChatRequest{
		Model: "qwen-turbo",
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}
	
	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	
	httpReq, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	var chatResp ChatResponse
	err = json.Unmarshal(body, &chatResp)
	if err != nil {
		return "", err
	}
	
	if len(chatResp.Choices) > 0 {
		return chatResp.Choices[0].Message.Content, nil
	}
	
	return "", nil
}

func (c *AIClient) AnalyzeOrders(orders []OrderInfo) (string, error) {
	prompt := "鍒嗘瀽浠ヤ笅璁㈠崟璁板綍锛屾彁鍙栫敤鎴烽ギ椋熷亸濂斤紝杩斿洖JSON鏍煎紡锛歿\"categories\":[],\"tastes\":[],\"price_range\":\"\",\"keywords\":[]}\\n\\n"
	
	for _, order := range orders {
		prompt += "璁㈠崟锛�?" + order.MerchantName + "锛岃彍鍝侊細" + order.Dishes + "锛屼环鏍硷細" + order.Price + "\\n"
	}
	
	return c.Chat(prompt)
}

func (c *AIClient) RecommendScore(userPref, merchant string) (float64, error) {
	prompt := "鐀��埛鍋忓ソ锛�" + userPref + "\\n鍟嗗淇℃伅锛�" + merchant + "\\n璇风粰鍑哄尮閰嶅害璇勫垎0-100锛屽彧杩斿洖鏁板�?"
	
	result, err := c.Chat(prompt)
	if err != nil {
		return 0, err
	}
	
	var score float64
	json.Unmarshal([]byte(result), &score)
	return score, nil
}

type OrderInfo struct {
	MerchantName string
	Dishes       string
	Price        string
}
