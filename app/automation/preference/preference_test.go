package preference

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"xiangchisha/internal/distributed/contracts"
	"xiangchisha/internal/distributed/service"
)

func TestPreference(t *testing.T) {
	ctx := context.Background()
	svc := service.NewUserService()
	username := fmt.Sprintf("preference_test_%d", time.Now().UnixNano())

	registerResp, err := svc.Register(ctx, &contracts.RegisterRequest{
		Username: username,
		Password: "123456",
	})
	if err != nil || registerResp.Code != 0 {
		t.Fatalf("注册失败：resp=%+v err=%v", registerResp, err)
	}

	loginResp, err := svc.Login(ctx, &contracts.LoginRequest{
		Username: username,
		Password: "123456",
	})
	if err != nil || loginResp.Code != 0 {
		t.Fatalf("登录失败：resp=%+v err=%v", loginResp, err)
	}

	t.Run("M1-UT-019_未填写偏好查询", func(t *testing.T) {
		resp, err := svc.GetPreference(ctx, &contracts.UserIDRequest{UserID: loginResp.UserID})
		if err != nil || resp.Code != 0 || resp.Data.HasPreference || resp.Data.Preference != nil {
			t.Fatalf("查询结果错误：resp=%+v err=%v", resp, err)
		}
	})

	request := &contracts.UpdatePreferenceRequest{
		UserID:       loginResp.UserID,
		Categories:   []string{"川菜"},
		PriceRange:   "20-40元",
		Tastes:       []string{"微辣"},
		Merchants:    []int64{1001},
		DishKeywords: []string{"鸡腿饭"},
		AvoidFoods:   []string{"香菜"},
		OrderTimes:   []int32{12},
	}

	t.Run("M1-UT-020_保存单个菜系", func(t *testing.T) {
		resp, err := svc.UpdatePreference(ctx, request)
		if err != nil || resp.Code != 0 {
			t.Fatalf("保存失败：resp=%+v err=%v", resp, err)
		}
	})

	t.Run("M1-UT-021_序列化偏好字段", func(t *testing.T) {
		resp, err := svc.GetPreference(ctx, &contracts.UserIDRequest{UserID: loginResp.UserID})
		if err != nil || resp.Code != 0 || !resp.Data.HasPreference || resp.Data.Preference == nil {
			t.Fatalf("查询失败：resp=%+v err=%v", resp, err)
		}
		got := resp.Data.Preference
		if !reflect.DeepEqual(got.Categories, request.Categories) ||
			got.PriceRange != request.PriceRange ||
			!reflect.DeepEqual(got.Tastes, request.Tastes) ||
			!reflect.DeepEqual(got.Merchants, request.Merchants) ||
			!reflect.DeepEqual(got.DishKeywords, request.DishKeywords) ||
			!reflect.DeepEqual(got.AvoidFoods, request.AvoidFoods) ||
			!reflect.DeepEqual(got.OrderTimes, request.OrderTimes) {
			t.Fatalf("偏好数据不一致：got=%+v", got)
		}
	})
}
