package account

import (
	"context"
	"fmt"
	"testing"
	"time"

	"xiangchisha/internal/distributed/contracts"
	"xiangchisha/internal/distributed/service"
)

func TestAccountQuery(t *testing.T) {
	ctx := context.Background()
	svc := service.NewUserService()
	username := fmt.Sprintf("account_test_%d", time.Now().UnixNano())

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

	t.Run("M1-UT-017_查询已注册用户", func(t *testing.T) {
		accountResp, err := svc.GetUserInfo(ctx, &contracts.UserIDRequest{UserID: loginResp.UserID})
		if err != nil || accountResp.Code != 0 {
			t.Fatalf("查询账户失败：resp=%+v err=%v", accountResp, err)
		}
		if accountResp.Data.Username != username || accountResp.Data.LocationPermission != "unset" {
			t.Fatalf("账户信息错误：%+v", accountResp.Data)
		}
	})

	t.Run("M1-UT-018_查询不存在用户", func(t *testing.T) {
		accountResp, err := svc.GetUserInfo(ctx, &contracts.UserIDRequest{UserID: 9999})
		if err != nil || accountResp.Code != 404 {
			t.Fatalf("查询不存在用户错误：resp=%+v err=%v", accountResp, err)
		}
	})
}
