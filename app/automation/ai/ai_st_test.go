package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const baseURL = "http://127.0.0.1:8080/api/v1"

type merchant struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Rating       float64  `json:"rating"`
	AvgPrice     float64  `json:"avg_price"`
	Distance     string   `json:"distance"`
	DeliveryTime int32    `json:"delivery_time"`
	Tags         []string `json:"tags"`
	Reason       string   `json:"reason"`
}

type recommendResult struct {
	Status        int
	Code          int        `json:"code"`
	Reply         string     `json:"reply"`
	IsOrderIntent bool       `json:"is_order_intent"`
	Merchants     []merchant `json:"merchants"`
	Elapsed       time.Duration
}

func TestAIFirstTen(t *testing.T) {
	client := &http.Client{Timeout: 300 * time.Second}

	t.Run("AI-ST-001", func(t *testing.T) {
		token := newUser(t, client, baseURL)
		got := recommend(t, client, baseURL, token, map[string]string{
			"requirement": "今晚想吃川菜，预算40元以内，30分钟内送到",
			"location":    "上海市杨浦区",
		})
		if got.Status != http.StatusOK || got.Code != 0 || !got.IsOrderIntent {
			t.Fatalf("响应错误：%+v", got)
		}
		if n := len(got.Merchants); n < 1 || n > 3 {
			t.Fatalf("商家数量=%d", n)
		}
		if strings.TrimSpace(got.Reply) == "" {
			t.Fatal("reply 为空")
		}
		assertMerchantFields(t, got.Merchants)
	})

	t.Run("AI-ST-002", func(t *testing.T) {
		token := newUser(t, client, baseURL)
		got := recommend(t, client, baseURL, token, map[string]string{
			"requirement": "I want a light lunch under 35 RMB with fast delivery",
			"location":    "上海市杨浦区",
		})
		if got.Status != http.StatusOK || got.Code != 0 || !got.IsOrderIntent || len(got.Merchants) == 0 {
			t.Fatalf("响应错误：%+v", got)
		}
		if got.Elapsed > 240*time.Second {
			t.Fatalf("响应过慢：%s", got.Elapsed)
		}
	})

	t.Run("AI-ST-003", func(t *testing.T) {
		token := newUser(t, client, baseURL)
		got := recommend(t, client, baseURL, token, map[string]string{
			"requirement": "有点饿了，想恰点不太贵的，别太辣，快点送到哈",
			"location":    "上海市杨浦区",
		})
		if got.Status != http.StatusOK || got.Code != 0 || !got.IsOrderIntent {
			t.Fatalf("响应错误：%+v", got)
		}
		spicy := 0
		for _, m := range got.Merchants {
			reason := m.Reason
			for _, neg := range []string{"别太辣", "不太辣", "不要辣", "不吃辣", "不辣", "非辣", "免辣"} {
				reason = strings.ReplaceAll(reason, neg, "")
			}
			m.Reason = reason
			if merchantTextHas(m, "辣", "川菜", "湘菜", "麻婆", "火锅", "串串") {
				spicy++
			}
		}
		if len(got.Merchants) > 0 && spicy == len(got.Merchants) {
			t.Fatalf("未优先排除辣味商家：%+v reply=%s", got.Merchants, got.Reply)
		}
	})

	t.Run("AI-ST-004", func(t *testing.T) {
		token := newUser(t, client, baseURL)
		got := recommend(t, client, baseURL, token, map[string]string{
			"requirement": "   ",
			"location":    "上海市杨浦区",
		})
		if got.Status != http.StatusOK || got.Code != 0 || got.IsOrderIntent || len(got.Merchants) != 0 {
			t.Fatalf("空白输入不应推荐商家：%+v", got)
		}
		if strings.TrimSpace(got.Reply) == "" {
			t.Fatal("缺少友好提示")
		}
	})

	t.Run("AI-ST-005", func(t *testing.T) {
		token := newUser(t, client, baseURL)
		requirement := strings.Repeat("今", 1000) + "预算30元，想吃清淡晚餐，40分钟内送达"
		got := recommend(t, client, baseURL, token, map[string]string{
			"requirement": requirement,
			"location":    "上海市杨浦区",
		})
		if got.Status != http.StatusOK || got.Code != 0 || !got.IsOrderIntent {
			t.Fatalf("响应错误：%+v", got)
		}
		if got.Elapsed > 240*time.Second {
			t.Fatalf("响应过慢：%s", got.Elapsed)
		}
		if strings.TrimSpace(got.Reply) == "" {
			t.Fatal("返回结构不完整")
		}
	})

	t.Run("AI-ST-006", func(t *testing.T) {
		token := newUser(t, client, baseURL)
		got := recommend(t, client, baseURL, token, map[string]string{
			"requirement": "午饭推荐一下，预算35元",
			"location":    "空候选测试区域",
		})
		if got.Status != http.StatusOK || got.Code != 0 {
			t.Fatalf("响应错误：%+v", got)
		}
		if strings.TrimSpace(got.Reply) == "" {
			t.Fatal("缺少说明")
		}
		known := map[int64]struct{}{1001: {}, 1002: {}, 1003: {}, 900001: {}, 900002: {}}
		for _, m := range got.Merchants {
			if _, ok := known[m.ID]; !ok {
				t.Fatalf("编造商家：%+v", m)
			}
		}
	})

	t.Run("AI-ST-007", func(t *testing.T) {
		fallbackURL := startFallbackGateway(t)
		token := newUser(t, client, fallbackURL)
		got := recommend(t, client, fallbackURL, token, map[string]string{
			"requirement": "想吃辣一点的川菜，预算50元",
			"location":    "上海市杨浦区",
		})
		if got.Status != http.StatusOK || got.Code != 0 || !got.IsOrderIntent || len(got.Merchants) == 0 {
			t.Fatalf("兜底失败：%+v", got)
		}
		if !strings.Contains(got.Reply, "筛选") && !strings.Contains(got.Reply, "暂未找到") {
			t.Fatalf("未走兜底文案：%s", got.Reply)
		}
	})

	t.Run("AI-ST-008", func(t *testing.T) {
		lowToken := newUser(t, client, baseURL)
		highToken := newUser(t, client, baseURL)
		low := recommend(t, client, baseURL, lowToken, map[string]string{
			"requirement": "想吃清淡午饭，预算20元",
			"location":    "上海市杨浦区",
		})
		high := recommend(t, client, baseURL, highToken, map[string]string{
			"requirement": "想吃清淡午饭，预算80元",
			"location":    "上海市杨浦区",
		})
		if low.Status != http.StatusOK || low.Code != 0 || !low.IsOrderIntent || len(low.Merchants) == 0 {
			t.Fatalf("低预算响应错误：%+v", low)
		}
		if high.Status != http.StatusOK || high.Code != 0 || !high.IsOrderIntent || len(high.Merchants) == 0 {
			t.Fatalf("高预算响应错误：%+v", high)
		}
		lowRating := avgRating(low.Merchants)
		highRating := avgRating(high.Merchants)
		if lowRating < 4 || lowRating+0.5 < highRating {
			t.Fatalf("低预算结果质量偏低：low=%v high=%v", low.Merchants, high.Merchants)
		}
		if avgPrice(low.Merchants) > 45 {
			t.Fatalf("低预算价格偏离：%+v", low.Merchants)
		}
	})

	t.Run("AI-ST-009", func(t *testing.T) {
		token := newUser(t, client, baseURL)
		got := recommend(t, client, baseURL, token, map[string]string{
			"requirement": "晚饭吃什么推荐一个",
			"location":    "上海市杨浦区",
		})
		if got.Status != http.StatusOK || got.Code != 0 || !got.IsOrderIntent || len(got.Merchants) == 0 {
			t.Fatalf("新用户响应错误：%+v", got)
		}
		cats := map[string]struct{}{}
		for _, m := range got.Merchants {
			if m.Category != "" {
				cats[m.Category] = struct{}{}
			}
		}
		if len(cats) < 2 && strings.TrimSpace(got.Reply) == "" {
			t.Fatalf("推荐多样性不足：%+v", got.Merchants)
		}
	})

	t.Run("AI-ST-010", func(t *testing.T) {
		token := newUser(t, client, baseURL)
		putJSON(t, client, baseURL, "/user/preference", token, map[string]interface{}{
			"avoid_foods": []string{"牛肉", "羊肉"},
		})
		got := recommend(t, client, baseURL, token, map[string]string{
			"requirement": "午饭推荐，别有牛肉羊肉",
			"location":    "上海市杨浦区",
		})
		if got.Status != http.StatusOK || got.Code != 0 {
			t.Fatalf("响应错误：%+v", got)
		}
		if len(got.Merchants) == 0 {
			if !strings.Contains(got.Reply, "暂无") && !strings.Contains(got.Reply, "匹配") {
				t.Fatal("候选不足时缺少说明")
			}
			return
		}
		for _, m := range got.Merchants {
			if merchantTextHas(m, "牛肉", "羊肉") {
				t.Fatalf("推荐了禁忌食材：%+v", m)
			}
		}
	})
}

func assertMerchantFields(t *testing.T, merchants []merchant) {
	t.Helper()
	for _, m := range merchants {
		if m.ID == 0 || m.Name == "" || m.Category == "" || m.Rating <= 0 || m.AvgPrice <= 0 || m.Distance == "" || m.DeliveryTime <= 0 {
			t.Fatalf("商家字段不完整：%+v", m)
		}
	}
}

func merchantTextHas(m merchant, words ...string) bool {
	text := m.Name + m.Category + m.Reason + strings.Join(m.Tags, "")
	for _, w := range words {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}

func avgRating(merchants []merchant) float64 {
	var sum float64
	for _, m := range merchants {
		sum += m.Rating
	}
	return sum / float64(len(merchants))
}

func avgPrice(merchants []merchant) float64 {
	var sum float64
	for _, m := range merchants {
		sum += m.AvgPrice
	}
	return sum / float64(len(merchants))
}

func newUser(t *testing.T, client *http.Client, api string) string {
	t.Helper()
	username := fmt.Sprintf("ai_st_%d", time.Now().UnixNano())
	postJSON(t, client, api, "/user/register", "", map[string]string{"username": username, "password": "123456"})
	login := postJSON(t, client, api, "/user/login", "", map[string]string{"username": username, "password": "123456"})
	token, _ := login["token"].(string)
	if token == "" {
		t.Fatalf("登录未返回令牌：%v", login)
	}
	return token
}

func recommend(t *testing.T, client *http.Client, api, token string, body map[string]string) recommendResult {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, api+"/recommend/get", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("请求推荐接口失败，请先启动服务：%v", err)
	}
	defer resp.Body.Close()
	var got recommendResult
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	got.Status = resp.StatusCode
	got.Elapsed = elapsed
	return got
}

func postJSON(t *testing.T, client *http.Client, api, path, token string, body interface{}) map[string]interface{} {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, api+path, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("请求 %s 失败，请先启动服务：%v", path, err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if code, _ := result["code"].(float64); code != 0 {
		t.Fatalf("%s 失败：%v", path, result)
	}
	return result
}

func putJSON(t *testing.T, client *http.Client, api, path, token string, body interface{}) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPut, api+path, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if code, _ := result["code"].(float64); code != 0 {
		t.Fatalf("%s 失败：%v", path, result)
	}
}

func startFallbackGateway(t *testing.T) string {
	t.Helper()
	appRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(appRoot, ".runtime", "bin", "gateway.exe")
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("gateway 不存在：%v", err)
	}
	cmd := exec.Command(bin)
	cmd.Dir = appRoot
	cmd.Env = append(os.Environ(), "GATEWAY_HTTP_ADDR=127.0.0.1:18081", "MODELSCOPE_BASE_URL=http://127.0.0.1:1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
	})
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:18081/")
		if err == nil {
			resp.Body.Close()
			return "http://127.0.0.1:18081/api/v1"
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatal("兜底网关启动失败")
	return ""
}
