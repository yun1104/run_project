package recommend

import (
	"context"
	"fmt"
	"testing"
	"time"

	"xiangchisha/internal/distributed/contracts"
	"xiangchisha/internal/distributed/service"
)

func TestRecommendation(t *testing.T) {
	svc := service.NewRecommendService()
	resp, err := svc.GetRecommendations(context.Background(), &contracts.RecommendRequest{
		UserID:      time.Now().UnixNano(),
		Requirement: "快餐",
		Location:    fmt.Sprintf("test-%d", time.Now().UnixNano()),
	})
	if err != nil || resp.Code != 0 {
		t.Fatalf("推荐失败：resp=%+v err=%v", resp, err)
	}

	t.Run("M1-UT-028_三路召回去重", func(t *testing.T) {
		ids := make(map[int64]struct{})
		for _, merchant := range resp.Merchants {
			if _, exists := ids[merchant.ID]; exists {
				t.Fatalf("存在重复商家：%d", merchant.ID)
			}
			ids[merchant.ID] = struct{}{}
		}
		if len(resp.Merchants) != 3 {
			t.Fatalf("商家数量错误：%d", len(resp.Merchants))
		}
	})

	t.Run("M1-UT-029_按评分价格排序", func(t *testing.T) {
		for i := 1; i < len(resp.Merchants); i++ {
			if resp.Merchants[i-1].Score < resp.Merchants[i].Score {
				t.Fatalf("推荐未按评分排序：%+v", resp.Merchants)
			}
		}
	})
}
