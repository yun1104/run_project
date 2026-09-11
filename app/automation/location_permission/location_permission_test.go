package locationpermission

import (
	"context"
	"fmt"
	"testing"
	"time"

	"xiangchisha/internal/distributed/contracts"
	"xiangchisha/internal/distributed/service"
)

func TestLocationPermission(t *testing.T) {
	ctx := context.Background()
	svc := service.NewUserService()
	username := fmt.Sprintf("location_test_%d", time.Now().UnixNano())

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

	validCases := []struct {
		id, input, want string
	}{
		{"M1-UT-022", "always", "always"},
		{"M1-UT-023", "denied", "denied"},
		{"M1-UT-024", "unset", "unset"},
		{"M1-UT-025", " ALWAYS ", "always"},
	}
	for _, tc := range validCases {
		t.Run(tc.id, func(t *testing.T) {
			resp, err := svc.UpdateLocationPermission(ctx, &contracts.UpdateLocationPermissionRequest{
				UserID:             loginResp.UserID,
				LocationPermission: tc.input,
			})
			if err != nil || resp.Code != 0 {
				t.Fatalf("更新失败：resp=%+v err=%v", resp, err)
			}
			got, err := svc.GetLocationPermission(ctx, &contracts.UserIDRequest{UserID: loginResp.UserID})
			if err != nil || got.Code != 0 || got.LocationPermission != tc.want {
				t.Fatalf("查询结果错误：resp=%+v err=%v", got, err)
			}
		})
	}

	for _, tc := range []struct {
		id, input string
	}{
		{"M1-UT-026", "once"},
		{"M1-UT-027", ""},
	} {
		t.Run(tc.id, func(t *testing.T) {
			resp, err := svc.UpdateLocationPermission(ctx, &contracts.UpdateLocationPermissionRequest{
				UserID:             loginResp.UserID,
				LocationPermission: tc.input,
			})
			if err != nil || resp.Code != 400 {
				t.Fatalf("非法权限应返回400：resp=%+v err=%v", resp, err)
			}
		})
	}
}
