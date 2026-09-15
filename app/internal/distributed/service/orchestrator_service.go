package service

import (
	"context"

	"xiangchisha/internal/distributed/contracts"
	"xiangchisha/internal/distributed/recommendrpc"
	"xiangchisha/internal/distributed/userrpc"
)

type OrchestratorService struct {
	userClient      *userrpc.Client
	recommendClient *recommendrpc.Client
}

func NewOrchestratorService(userClient *userrpc.Client, recommendClient *recommendrpc.Client) *OrchestratorService {
	return &OrchestratorService{
		userClient:      userClient,
		recommendClient: recommendClient,
	}
}

func (s *OrchestratorService) Register(ctx context.Context, req *contracts.RegisterRequest) (*contracts.BaseResponse, error) {
	return s.userClient.Register(ctx, req)
}

func (s *OrchestratorService) Login(ctx context.Context, req *contracts.LoginRequest) (*contracts.LoginResponse, error) {
	return s.userClient.Login(ctx, req)
}

func (s *OrchestratorService) ValidateToken(ctx context.Context, req *contracts.ValidateTokenRequest) (*contracts.ValidateTokenResponse, error) {
	return s.userClient.ValidateToken(ctx, req)
}

func (s *OrchestratorService) GetMe(ctx context.Context, req *contracts.UserIDRequest) (*contracts.MeResponse, error) {
	return s.userClient.GetUserInfo(ctx, req)
}

func (s *OrchestratorService) GetPreference(ctx context.Context, req *contracts.UserIDRequest) (*contracts.PreferenceResponse, error) {
	return s.userClient.GetPreference(ctx, req)
}

func (s *OrchestratorService) UpdatePreference(ctx context.Context, req *contracts.UpdatePreferenceRequest) (*contracts.BaseResponse, error) {
	return s.userClient.UpdatePreference(ctx, req)
}

func (s *OrchestratorService) GetLocationPermission(ctx context.Context, req *contracts.UserIDRequest) (*contracts.LocationPermissionResponse, error) {
	return s.userClient.GetLocationPermission(ctx, req)
}

func (s *OrchestratorService) UpdateLocationPermission(ctx context.Context, req *contracts.UpdateLocationPermissionRequest) (*contracts.BaseResponse, error) {
	return s.userClient.UpdateLocationPermission(ctx, req)
}

func (s *OrchestratorService) GetRecommendations(ctx context.Context, req *contracts.RecommendRequest) (*contracts.RecommendResponse, error) {
	return s.recommendClient.GetRecommendations(ctx, req)
}
