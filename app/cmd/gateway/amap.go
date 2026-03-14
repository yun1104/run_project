package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"xiangchisha/internal/distributed/contracts"
)

type amapNearbyFood struct {
	POIID          string  `json:"poi_id"`
	Name           string  `json:"name"`
	Address        string  `json:"address"`
	DistanceMeters int     `json:"distance_meters"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	Category       string  `json:"category"`
	Rating         float64 `json:"rating"`
	AvgPrice       float64 `json:"avg_price"`
	Dishes         []string `json:"dishes"`
}

type amapAroundResponse struct {
	Status string `json:"status"`
	Info   string `json:"info"`
	Pois   []struct {
		ID       string                 `json:"id"`
		Name     string                 `json:"name"`
		Address  string                 `json:"address"`
		Distance string                 `json:"distance"`
		Type     string                 `json:"type"`
		Location string                 `json:"location"`
		Business map[string]interface{} `json:"business"`
	} `json:"pois"`
}

func getAmapAPIKey() string {
	if key := strings.TrimSpace(os.Getenv("AMAP_API_KEY")); key != "" {
		return key
	}
	if key := strings.TrimSpace(os.Getenv("GAODE_API_KEY")); key != "" {
		return key
	}
	if key := readAPIKeyFromEnvFile(".env", "AMAP_API_KEY"); key != "" {
		return key
	}
	if key := readAPIKeyFromEnvFile("../.env", "AMAP_API_KEY"); key != "" {
		return key
	}
	if key := readAPIKeyFromEnvFile(".env.example", "AMAP_API_KEY"); key != "" {
		return key
	}
	if key := readAPIKeyFromEnvFile("../.env.example", "AMAP_API_KEY"); key != "" {
		return key
	}
	return ""
}

func readAPIKeyFromEnvFile(path, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	prefix := key + "="
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		val = strings.Trim(val, `"'`)
		if val != "" {
			return val
		}
	}
	return ""
}

func fetchNearbyFoodsFromAmap(ctx context.Context, latitude, longitude float64, radius, limit int) ([]amapNearbyFood, error) {
	if math.Abs(latitude) > 90 || math.Abs(longitude) > 180 {
		return nil, fmt.Errorf("invalid location")
	}
	apiKey := getAmapAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("missing AMAP_API_KEY")
	}
	if radius <= 0 {
		radius = 3000
	}
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}

	params := url.Values{}
	params.Set("key", apiKey)
	params.Set("location", fmt.Sprintf("%.6f,%.6f", longitude, latitude))
	params.Set("keywords", "美食")
	params.Set("types", "050000")
	params.Set("radius", strconv.Itoa(radius))
	params.Set("sortrule", "distance")
	params.Set("page_size", strconv.Itoa(limit))
	params.Set("page_num", "1")
	params.Set("show_fields", "business")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://restapi.amap.com/v5/place/around?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("amap http status: %d", resp.StatusCode)
	}

	var out amapAroundResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Status != "1" {
		return nil, fmt.Errorf("amap api failed: %s", out.Info)
	}

	items := make([]amapNearbyFood, 0, len(out.Pois))
	for _, poi := range out.Pois {
		lon, lat := parseAmapLocation(poi.Location)
		dist, _ := strconv.Atoi(strings.TrimSpace(poi.Distance))
		rating, avgPrice, tags := parseAmapBusiness(poi.Business)
		category := strings.TrimSpace(strings.Split(poi.Type, ";")[0])
		dishes := extractDishes(poi.Name, poi.Type, tags)
		items = append(items, amapNearbyFood{
			POIID:          strings.TrimSpace(poi.ID),
			Name:           strings.TrimSpace(poi.Name),
			Address:        strings.TrimSpace(poi.Address),
			DistanceMeters: dist,
			Latitude:       lat,
			Longitude:      lon,
			Category:       category,
			Rating:         rating,
			AvgPrice:       avgPrice,
			Dishes:         dishes,
		})
	}
	return items, nil
}

func parseAmapBusiness(biz map[string]interface{}) (float64, float64, string) {
	if biz == nil {
		return 0, 0, ""
	}
	rating, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(biz["rating"])), 64)
	cost, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(biz["cost"])), 64)
	tag := strings.TrimSpace(fmt.Sprint(biz["tag"]))
	return rating, cost, tag
}

func parseAmapLocation(loc string) (float64, float64) {
	parts := strings.Split(strings.TrimSpace(loc), ",")
	if len(parts) != 2 {
		return 0, 0
	}
	lon, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lat, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	return lon, lat
}

func mergeCandidatesWithNearby(nearby []amapNearbyFood) []contracts.Merchant {
	out := make([]contracts.Merchant, 0, len(nearby))
	seen := map[int64]struct{}{}
	for _, n := range nearby {
		id := syntheticMerchantIDFromPOI(n.POIID, n.Name, n.Address)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		reason := fmt.Sprintf("附近%d米", n.DistanceMeters)
		out = append(out, contracts.Merchant{
			ID:           id,
			Name:         n.Name,
			Category:     n.Category,
			Rating:       n.Rating,
			AvgPrice:     n.AvgPrice,
			Distance:     fmt.Sprintf("%dm", n.DistanceMeters),
			DeliveryTime: int32(estimateDeliveryTime(n.DistanceMeters)),
			Tags:         n.Dishes,
			Reason:       reason,
		})
	}
	return out
}

func syntheticMerchantIDFromPOI(poiID, name, address string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.TrimSpace(poiID) + "|" + strings.TrimSpace(name) + "|" + strings.TrimSpace(address)))
	return 900000000 + int64(h.Sum32()%99999999)
}

func estimateDeliveryTime(distanceMeters int) int {
	if distanceMeters <= 0 {
		return 30
	}
	eta := 15 + distanceMeters/120
	if eta < 15 {
		eta = 15
	}
	if eta > 60 {
		eta = 60
	}
	return eta
}

func extractDishes(name, typ, tag string) []string {
	parts := splitDishTokens(tag)
	text := strings.ToLower(name + " " + typ + " " + tag)
	appendIf := func(keyword string, vals ...string) {
		if strings.Contains(text, keyword) {
			parts = append(parts, vals...)
		}
	}
	appendIf("川", "麻婆豆腐", "宫保鸡丁")
	appendIf("湘", "剁椒鱼头", "小炒肉")
	appendIf("粤", "烧腊饭", "白切鸡")
	appendIf("火锅", "牛肉卷", "虾滑")
	appendIf("烧烤", "羊肉串", "烤茄子")
	appendIf("面", "牛肉面", "炸酱面")
	appendIf("米线", "过桥米线")
	appendIf("麻辣烫", "麻辣烫")
	appendIf("轻食", "鸡胸肉沙拉", "藜麦能量碗")
	appendIf("汉堡", "牛肉汉堡", "薯条")
	appendIf("披萨", "榴莲披萨", "培根披萨")

	uniq := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		uniq = append(uniq, p)
		if len(uniq) >= 6 {
			break
		}
	}
	return uniq
}

func splitDishTokens(s string) []string {
	replacer := strings.NewReplacer("|", ",", ";", ",", "，", ",", "、", ",", "/", ",")
	s = replacer.Replace(s)
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		lower := strings.ToLower(item)
		if lower == "<nil>" || lower == "nil" || lower == "null" {
			continue
		}
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
