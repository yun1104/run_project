package service

import (
	"context"
	"encoding/json"
	"fmt"
	"xiangchisha/pkg/cache"
	"sync"
	"time"
)

type RecommendService struct {
	aiClient interface{}
}

func NewRecommendService() *RecommendService {
	return &RecommendService{}
}

func (s *RecommendService) GetRecommendations(ctx context.Context, userID int64, requirement, location string) ([]Merchant, error) {
	cacheKey := fmt.Sprintf("recommend:%d:%s", userID, requirement)
	
	var merchants []Merchant
	err := cache.Get(ctx, cacheKey, &merchants)
	if err == nil {
		return merchants, nil
	}
	
	merchants = s.multiRecall(ctx, userID, location)
	
	s.aiRank(ctx, merchants, requirement)
	
	if len(merchants) > 20 {
		merchants = merchants[:20]
	}
	
	cache.Set(ctx, cacheKey, merchants, 10*time.Minute)
	
	return merchants, nil
}

func (s *RecommendService) multiRecall(ctx context.Context, userID int64, location string) []Merchant {
	var wg sync.WaitGroup
	ch := make(chan []Merchant, 3)
	
	wg.Add(3)
	
	go func() {
		defer wg.Done()
		ch <- s.preferenceRecall(ctx, userID)
	}()
	
	go func() {
		defer wg.Done()
		ch <- s.hotRecall(ctx, location)
	}()
	
	go func() {
		defer wg.Done()
		ch <- s.cfRecall(ctx, userID)
	}()
	
	go func() {
		wg.Wait()
		close(ch)
	}()
	
	merchants := make([]Merchant, 0)
	merchantMap := make(map[int64]bool)
	
	for ms := range ch {
		for _, m := range ms {
			if !merchantMap[m.ID] {
				merchantMap[m.ID] = true
				merchants = append(merchants, m)
			}
		}
	}
	
	return merchants
}

func (s *RecommendService) preferenceRecall(ctx context.Context, userID int64) []Merchant {
	key := fmt.Sprintf("recall:pref:%d", userID)
	
	var merchants []Merchant
	cache.Get(ctx, key, &merchants)
	
	return merchants
}

func (s *RecommendService) hotRecall(ctx context.Context, location string) []Merchant {
	key := fmt.Sprintf("recall:hot:%s", location)
	
	var merchants []Merchant
	data, _ := cache.GetClient().ZRevRange(ctx, key, 0, 49).Result()
	
	for _, item := range data {
		var m Merchant
		json.Unmarshal([]byte(item), &m)
		merchants = append(merchants, m)
	}
	
	return merchants
}

func (s *RecommendService) cfRecall(ctx context.Context, userID int64) []Merchant {
	key := fmt.Sprintf("recall:cf:%d", userID)
	
	var merchants []Merchant
	cache.Get(ctx, key, &merchants)
	
	return merchants
}

func (s *RecommendService) aiRank(ctx context.Context, merchants []Merchant, requirement string) {
	for i := range merchants {
		merchants[i].Score = float64(100 - i)
	}
}

type Merchant struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Rating       float64  `json:"rating"`
	AvgPrice     float64  `json:"avg_price"`
	Distance     string   `json:"distance"`
	DeliveryTime int      `json:"delivery_time"`
	Tags         []string `json:"tags"`
	Reason       string   `json:"reason"`
	Score        float64  `json:"score"`
}
