package interfaceapi

import (
	"net/http"
	"testing"
	"time"
)

func TestM1IT030UnauthenticatedPreference(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://127.0.0.1:8080/api/v1/user/preference")
	if err != nil {
		t.Fatalf("请求接口失败，请先启动服务：%v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("未认证请求应返回401，实际返回%d", resp.StatusCode)
	}
}
