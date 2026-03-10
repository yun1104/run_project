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
	"sort"
	"strconv"
	"strings"
	"time"
)

type AmapNearbyFood struct {
	POIID          string   `json:"poi_id"`
	Name           string   `json:"name"`
	Address        string   `json:"address"`
	DistanceMeters int      `json:"distance_meters"`
	Latitude       float64  `json:"latitude"`
	Longitude      float64  `json:"longitude"`
	Category       string   `json:"category"`
	Rating         float64  `json:"rating"`
	AvgPrice       float64  `json:"avg_price"`
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
	return strings.TrimSpace(os.Getenv("GAODE_API_KEY"))
}

func fetchNearbyFoodsFromAmap(ctx context.Context, latitude, longitude float64, radius, limit int) ([]AmapNearbyFood, error) {
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

	items := make([]AmapNearbyFood, 0, len(out.Pois))
	for _, poi := range out.Pois {
		lon, lat := parseAmapLocation(poi.Location)
		dist, _ := strconv.Atoi(strings.TrimSpace(poi.Distance))
		rating, avgPrice, tags := parseAmapBusiness(poi.Business)
		dishes := extractDishes(poi.Name, poi.Type, tags)
		category := strings.TrimSpace(strings.Split(poi.Type, ";")[0])
		if avgPrice <= 0 {
			avgPrice = estimateAvgPrice(poi.Type + " " + poi.Name)
		}
		items = append(items, AmapNearbyFood{
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
	sort.Slice(items, func(i, j int) bool {
		return items[i].DistanceMeters < items[j].DistanceMeters
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func mergeCandidatesWithNearby(base []Merchant, nearby []AmapNearbyFood) []Merchant {
	if len(nearby) == 0 {
		return base
	}
	out := make([]Merchant, 0, len(base)+len(nearby))
	seen := map[int64]struct{}{}
	for _, m := range base {
		out = append(out, m)
		seen[m.ID] = struct{}{}
	}
	for _, n := range nearby {
		id := syntheticMerchantIDFromPOI(n.POIID, n.Name, n.Address)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		reason := fmt.Sprintf("附近%d米", n.DistanceMeters)
		if len(n.Dishes) > 0 {
			reason += "，热门菜品：" + strings.Join(n.Dishes[:minInt(len(n.Dishes), 3)], "、")
		}
		out = append(out, Merchant{
			ID:                id,
			Name:              n.Name,
			Category:          n.Category,
			Rating:            n.Rating,
			AvgPrice:          n.AvgPrice,
			DeliveryTime:      estimateDeliveryTime(n.DistanceMeters),
			Reason:            reason,
			RecommendedDishes: n.Dishes,
		})
	}
	return out
}

func parseAmapBusiness(biz map[string]interface{}) (float64, float64, string) {
	if biz == nil {
		return 0, 0, ""
	}
	rating, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(biz["rating"])), 64)
	tag := strings.TrimSpace(fmt.Sprint(biz["tag"]))

	costStr := strings.TrimSpace(fmt.Sprint(biz["cost"]))
	var cost float64
	if costStr != "" && costStr != "<nil>" && costStr != "nil" && costStr != "0" {
		cost, _ = strconv.ParseFloat(costStr, 64)
	}
	return rating, cost, tag
}

// estimateAvgPrice 根据品类估算人均价格，仅在高德未提供时使用
func estimateAvgPrice(category string) float64 {
	c := strings.ToLower(category)
	switch {
	case strings.Contains(c, "火锅"):
		return 60
	case strings.Contains(c, "烧烤"):
		return 50
	case strings.Contains(c, "日料") || strings.Contains(c, "日本"):
		return 80
	case strings.Contains(c, "西餐"):
		return 70
	case strings.Contains(c, "海鲜"):
		return 80
	case strings.Contains(c, "粤菜") || strings.Contains(c, "粤"):
		return 45
	case strings.Contains(c, "川菜") || strings.Contains(c, "湘菜"):
		return 40
	case strings.Contains(c, "快餐") || strings.Contains(c, "简餐"):
		return 20
	case strings.Contains(c, "面食") || strings.Contains(c, "米粉") || strings.Contains(c, "米线"):
		return 18
	case strings.Contains(c, "轻食"):
		return 35
	default:
		return 35
	}
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

func extractDishes(name, typ, tag string) []string {
	parts := splitDishTokens(tag)
	text := strings.ToLower(name + " " + typ + " " + tag)

	appendIf := func(dishes *[]string, keyword string, vals ...string) {
		if strings.Contains(text, keyword) {
			*dishes = append(*dishes, vals...)
		}
	}

	appendIf(&parts, "川", "麻婆豆腐", "宫保鸡丁")
	appendIf(&parts, "湘", "剁椒鱼头", "小炒肉")
	appendIf(&parts, "粤", "烧腊饭", "白切鸡")
	appendIf(&parts, "火锅", "牛肉卷", "虾滑")
	appendIf(&parts, "烧烤", "羊肉串", "烤茄子")
	appendIf(&parts, "面", "牛肉面", "炸酱面")
	appendIf(&parts, "米线", "过桥米线")
	appendIf(&parts, "麻辣烫", "麻辣烫")
	appendIf(&parts, "轻食", "鸡胸肉沙拉", "藜麦能量碗")
	appendIf(&parts, "汉堡", "牛肉汉堡", "薯条")
	appendIf(&parts, "披萨", "榴莲披萨", "培根披萨")

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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type amapRegeoResponse struct {
	Status string `json:"status"`
	Regeocode struct {
		AddressComponent struct {
			Province string `json:"province"`
			City     string `json:"city"`
			District string `json:"district"`
		} `json:"addressComponent"`
	} `json:"regeocode"`
}

var cityToPinyin = map[string]string{
	"北京": "beijing", "北京市": "beijing",
	"上海": "shanghai", "上海市": "shanghai",
	"天津": "tianjin", "天津市": "tianjin",
	"重庆": "chongqing", "重庆市": "chongqing",
	"广州": "guangzhou", "广州市": "guangzhou",
	"深圳": "shenzhen", "深圳市": "shenzhen",
	"杭州": "hangzhou", "杭州市": "hangzhou",
	"成都": "chengdu", "成都市": "chengdu",
	"武汉": "wuhan", "武汉市": "wuhan",
	"西安": "xian", "西安市": "xian",
	"南京": "nanjing", "南京市": "nanjing",
	"苏州": "suzhou", "苏州市": "suzhou",
	"青岛": "qingdao", "青岛市": "qingdao",
	"大连": "dalian", "大连市": "dalian",
	"长沙": "changsha", "长沙市": "changsha",
	"郑州": "zhengzhou", "郑州市": "zhengzhou",
	"济南": "jinan", "济南市": "jinan",
	"福州": "fuzhou", "福州市": "fuzhou",
	"厦门": "xiamen", "厦门市": "xiamen",
	"沈阳": "shenyang", "沈阳市": "shenyang",
	"哈尔滨": "haerbin", "哈尔滨市": "haerbin",
	"无锡": "wuxi", "无锡市": "wuxi",
	"宁波": "ningbo", "宁波市": "ningbo",
	"东莞": "dongguan", "东莞市": "dongguan",
	"佛山": "foshan", "佛山市": "foshan",
	"昆明": "kunming", "昆明市": "kunming",
	"合肥": "hefei", "合肥市": "hefei",
	"南昌": "nanchang", "南昌市": "nanchang",
	"石家庄": "shijiazhuang", "石家庄市": "shijiazhuang",
	"长春": "changchun", "长春市": "changchun",
	"太原": "taiyuan", "太原市": "taiyuan",
	"珠海": "zhuhai", "珠海市": "zhuhai",
	"温州": "wenzhou", "温州市": "wenzhou",
	"常州": "changzhou", "常州市": "changzhou",
	"绍兴": "shaoxing", "绍兴市": "shaoxing",
	"嘉兴": "jiaxing", "嘉兴市": "jiaxing",
}

func fetchCityPinyinFromAmap(ctx context.Context, latitude, longitude float64) string {
	if math.Abs(latitude) > 90 || math.Abs(longitude) > 180 {
		return ""
	}
	apiKey := getAmapAPIKey()
	if apiKey == "" {
		return ""
	}
	params := url.Values{}
	params.Set("key", apiKey)
	params.Set("location", fmt.Sprintf("%.6f,%.6f", longitude, latitude))
	params.Set("extensions", "base")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://restapi.amap.com/v3/geocode/regeo?"+params.Encode(), nil)
	if err != nil {
		return ""
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	var out amapRegeoResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ""
	}
	if out.Status != "1" {
		return ""
	}
	ac := out.Regeocode.AddressComponent
	city := strings.TrimSpace(ac.City)
	province := strings.TrimSpace(ac.Province)
	if city == "" || city == "[]" {
		city = province
	}
	if city == "" {
		return ""
	}
	city = strings.TrimSuffix(city, "市")
	province = strings.TrimSuffix(province, "市")
	if p, ok := cityToPinyin[city]; ok {
		return p
	}
	if p, ok := cityToPinyin[city+"市"]; ok {
		return p
	}
	if p, ok := cityToPinyin[province]; ok {
		return p
	}
	if p, ok := cityToPinyin[province+"市"]; ok {
		return p
	}
	return ""
}
