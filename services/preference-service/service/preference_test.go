package service

import (
	"context"
	"testing"
)

func TestAnalyzeOrders(t *testing.T) {
	svc := NewPreferenceService()

	empty, err := svc.AnalyzeOrders(context.Background(), nil)
	if err != nil {
		t.Fatalf("empty orders returned error: %v", err)
	}
	if empty != nil {
		t.Fatalf("empty orders should return nil preference, got %+v", empty)
	}

	pref, err := svc.AnalyzeOrders(context.Background(), []Order{
		{MerchantName: "轻食研究所", Dishes: []string{"鸡胸肉沙拉"}, Price: 35, OrderTime: "12:00"},
	})
	if err != nil {
		t.Fatalf("non-empty orders returned error: %v", err)
	}
	if pref == nil {
		t.Fatal("non-empty orders should return preference")
	}
	if pref.PriceRange == "" || len(pref.Categories) == 0 || len(pref.Tastes) == 0 {
		t.Fatalf("preference should contain analyzed fields, got %+v", pref)
	}
}

func TestMergePreference(t *testing.T) {
	svc := NewPreferenceService()
	oldPref := &UserPreference{
		Categories:   []string{"川菜", "快餐"},
		PriceRange:   "20-40",
		Tastes:       []string{"微辣"},
		Merchants:    []int64{10001},
		DishKeywords: []string{"牛肉饭"},
		AvoidFoods:   []string{"花生"},
		OrderTimes:   []int{12},
	}
	newPref := &UserPreference{
		Categories:   []string{"快餐", "轻食"},
		PriceRange:   "30-50",
		Tastes:       []string{"微辣", "清淡"},
		Merchants:    []int64{10001, 20001},
		DishKeywords: []string{"沙拉"},
		AvoidFoods:   []string{"海鲜"},
		OrderTimes:   []int{12, 18},
	}

	merged := svc.MergePreference(oldPref, newPref)
	if merged.PriceRange != "30-50" {
		t.Fatalf("price range = %q, want new preference value", merged.PriceRange)
	}
	assertContainsAll(t, merged.Categories, []string{"川菜", "快餐", "轻食"})
	assertContainsAll(t, merged.Tastes, []string{"微辣", "清淡"})
	assertContainsAll(t, merged.DishKeywords, []string{"牛肉饭", "沙拉"})
	assertContainsAll(t, merged.AvoidFoods, []string{"花生", "海鲜"})
	if len(merged.Merchants) != 2 {
		t.Fatalf("merchant merge should dedupe to 2 values, got %+v", merged.Merchants)
	}
	if len(merged.OrderTimes) != 2 {
		t.Fatalf("order time merge should dedupe to 2 values, got %+v", merged.OrderTimes)
	}
}

func TestMergeHelpersDeduplicate(t *testing.T) {
	if got := mergeStringSlice([]string{"a", "b"}, []string{"b", "c"}); len(got) != 3 {
		t.Fatalf("mergeStringSlice length = %d, want 3: %+v", len(got), got)
	}
	if got := mergeInt64Slice([]int64{1, 2}, []int64{2, 3}); len(got) != 3 {
		t.Fatalf("mergeInt64Slice length = %d, want 3: %+v", len(got), got)
	}
	if got := mergeIntSlice([]int{12, 18}, []int{18, 20}); len(got) != 3 {
		t.Fatalf("mergeIntSlice length = %d, want 3: %+v", len(got), got)
	}
}

func assertContainsAll(t *testing.T, got []string, want []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, item := range got {
		seen[item] = true
	}
	for _, item := range want {
		if !seen[item] {
			t.Fatalf("slice %+v does not contain %q", got, item)
		}
	}
}
