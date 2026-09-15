package service

import (
	"context"
	"encoding/json"
)

type PreferenceService struct {
	aiClient interface{}
}

func NewPreferenceService() *PreferenceService {
	return &PreferenceService{}
}

func (s *PreferenceService) AnalyzeOrders(ctx context.Context, orders []Order) (*UserPreference, error) {
	if len(orders) == 0 {
		return nil, nil
	}
	
	prompt := s.buildPrompt(orders)
	_ = prompt
	
	pref := &UserPreference{
		Categories:   []string{"宸濊�?", "蹇�?"},
		PriceRange:   "20-50",
		Tastes:       []string{"杈�", "棣�"},
		DishKeywords: []string{"绫抽�?", "楦¤�?"},
		OrderTimes:   []int{12, 18},
	}
	
	return pref, nil
}

func (s *PreferenceService) buildPrompt(orders []Order) string {
	prompt := "鍒嗘瀽浠ヤ笅璁㈠崟璁板綍锛屾彁鍙栫敤鎴烽ギ椋熷亸濂斤紝杩斿洖JSON鏍煎紡锛歿\"categories\":[],\"tastes\":[],\"price_range\":\"\",\"keywords\":[]}\\n\\n"
	
	for _, order := range orders {
		data, _ := json.Marshal(order)
		prompt += string(data) + "\\n"
	}
	
	return prompt
}

func (s *PreferenceService) MergePreference(old, new *UserPreference) *UserPreference {
	merged := &UserPreference{
		Categories:   mergeStringSlice(old.Categories, new.Categories),
		PriceRange:   new.PriceRange,
		Tastes:       mergeStringSlice(old.Tastes, new.Tastes),
		Merchants:    mergeInt64Slice(old.Merchants, new.Merchants),
		DishKeywords: mergeStringSlice(old.DishKeywords, new.DishKeywords),
		AvoidFoods:   mergeStringSlice(old.AvoidFoods, new.AvoidFoods),
		OrderTimes:   mergeIntSlice(old.OrderTimes, new.OrderTimes),
	}
	
	return merged
}

type Order struct {
	MerchantName string
	Dishes       []string
	Price        float64
	OrderTime    string
}

type UserPreference struct {
	Categories   []string
	PriceRange   string
	Tastes       []string
	Merchants    []int64
	DishKeywords []string
	AvoidFoods   []string
	OrderTimes   []int
}

func mergeStringSlice(a, b []string) []string {
	m := make(map[string]bool)
	for _, s := range a {
		m[s] = true
	}
	for _, s := range b {
		m[s] = true
	}
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

func mergeInt64Slice(a, b []int64) []int64 {
	m := make(map[int64]bool)
	for _, n := range a {
		m[n] = true
	}
	for _, n := range b {
		m[n] = true
	}
	result := make([]int64, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

func mergeIntSlice(a, b []int) []int {
	m := make(map[int]bool)
	for _, n := range a {
		m[n] = true
	}
	for _, n := range b {
		m[n] = true
	}
	result := make([]int, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
