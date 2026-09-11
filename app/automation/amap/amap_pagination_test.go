package amap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

const baseURL = "http://127.0.0.1:8080/api/v1"

func TestM1UT034NearbyFoodsPagination(t *testing.T) {
	username := fmt.Sprintf("amap_test_%d", time.Now().UnixNano())
	postJSON(t, "/user/register", map[string]string{"username": username, "password": "123456"})
	login := postJSON(t, "/user/login", map[string]string{"username": username, "password": "123456"})
	token, _ := login["token"].(string)
	if token == "" {
		t.Fatal("登录未返回令牌")
	}

	postAuthorizedJSON(t, "/user/location", token, map[string]interface{}{"latitude": 39.908, "longitude": 116.397, "source": "automation"})
	req, err := http.NewRequest(http.MethodGet, baseURL+"/user/location/current?with_nearby=true&limit=100", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result struct {
		Code int `json:"code"`
		Data struct {
			NearbyFoods []json.RawMessage `json:"nearby_foods"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || len(result.Data.NearbyFoods) <= 8 {
		t.Fatalf("附近商家数量=%d，期望大于8", len(result.Data.NearbyFoods))
	}
}

func postJSON(t *testing.T, path string, body interface{}) map[string]interface{} {
	return postAuthorizedJSON(t, path, "", body)
}

func postAuthorizedJSON(t *testing.T, path, token string, body interface{}) map[string]interface{} {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
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
	return result
}
