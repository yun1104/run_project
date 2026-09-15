package apprpc

import (
	"context"
	"google.golang.org/grpc"

	"xiangchisha/internal/distributed/contracts"
	_ "xiangchisha/internal/rpcjson"
)

const ServiceName = "app.OrchestratorService"

type Service interface {
	Register(context.Context, *contracts.RegisterRequest) (*contracts.BaseResponse, error)
	Login(context.Context, *contracts.LoginRequest) (*contracts.LoginResponse, error)
	ValidateToken(context.Context, *contracts.ValidateTokenRequest) (*contracts.ValidateTokenResponse, error)
	GetMe(context.Context, *contracts.UserIDRequest) (*contracts.MeResponse, error)
	GetPreference(context.Context, *contracts.UserIDRequest) (*contracts.PreferenceResponse, error)
	UpdatePreference(context.Context, *contracts.UpdatePreferenceRequest) (*contracts.BaseResponse, error)
	GetLocationPermission(context.Context, *contracts.UserIDRequest) (*contracts.LocationPermissionResponse, error)
	UpdateLocationPermission(context.Context, *contracts.UpdateLocationPermissionRequest) (*contracts.BaseResponse, error)
	GetRecommendations(context.Context, *contracts.RecommendRequest) (*contracts.RecommendResponse, error)
}

func Register(server *grpc.Server, impl Service) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: ServiceName,
		HandlerType: (*Service)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Register", Handler: wrapRegister(impl)},
			{MethodName: "Login", Handler: wrapLogin(impl)},
			{MethodName: "ValidateToken", Handler: wrapValidateToken(impl)},
			{MethodName: "GetMe", Handler: wrapGetMe(impl)},
			{MethodName: "GetPreference", Handler: wrapGetPreference(impl)},
			{MethodName: "UpdatePreference", Handler: wrapUpdatePreference(impl)},
			{MethodName: "GetLocationPermission", Handler: wrapGetLocationPermission(impl)},
			{MethodName: "UpdateLocationPermission", Handler: wrapUpdateLocationPermission(impl)},
			{MethodName: "GetRecommendations", Handler: wrapGetRecommendations(impl)},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "proto/orchestrator.proto",
	}, impl)
}

func unaryHandler[T any](fullMethod string, impl func(context.Context, *T) (interface{}, error)) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		in := new(T)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: fullMethod}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return impl(ctx, req.(*T))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func wrapRegister(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return unaryHandler[contracts.RegisterRequest]("/"+ServiceName+"/Register", func(ctx context.Context, req *contracts.RegisterRequest) (interface{}, error) {
		return impl.Register(ctx, req)
	})
}

func wrapLogin(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return unaryHandler[contracts.LoginRequest]("/"+ServiceName+"/Login", func(ctx context.Context, req *contracts.LoginRequest) (interface{}, error) {
		return impl.Login(ctx, req)
	})
}

func wrapValidateToken(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return unaryHandler[contracts.ValidateTokenRequest]("/"+ServiceName+"/ValidateToken", func(ctx context.Context, req *contracts.ValidateTokenRequest) (interface{}, error) {
		return impl.ValidateToken(ctx, req)
	})
}

func wrapGetMe(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return unaryHandler[contracts.UserIDRequest]("/"+ServiceName+"/GetMe", func(ctx context.Context, req *contracts.UserIDRequest) (interface{}, error) {
		return impl.GetMe(ctx, req)
	})
}

func wrapGetPreference(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return unaryHandler[contracts.UserIDRequest]("/"+ServiceName+"/GetPreference", func(ctx context.Context, req *contracts.UserIDRequest) (interface{}, error) {
		return impl.GetPreference(ctx, req)
	})
}

func wrapUpdatePreference(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return unaryHandler[contracts.UpdatePreferenceRequest]("/"+ServiceName+"/UpdatePreference", func(ctx context.Context, req *contracts.UpdatePreferenceRequest) (interface{}, error) {
		return impl.UpdatePreference(ctx, req)
	})
}

func wrapGetLocationPermission(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return unaryHandler[contracts.UserIDRequest]("/"+ServiceName+"/GetLocationPermission", func(ctx context.Context, req *contracts.UserIDRequest) (interface{}, error) {
		return impl.GetLocationPermission(ctx, req)
	})
}

func wrapUpdateLocationPermission(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return unaryHandler[contracts.UpdateLocationPermissionRequest]("/"+ServiceName+"/UpdateLocationPermission", func(ctx context.Context, req *contracts.UpdateLocationPermissionRequest) (interface{}, error) {
		return impl.UpdateLocationPermission(ctx, req)
	})
}

func wrapGetRecommendations(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return unaryHandler[contracts.RecommendRequest]("/"+ServiceName+"/GetRecommendations", func(ctx context.Context, req *contracts.RecommendRequest) (interface{}, error) {
		return impl.GetRecommendations(ctx, req)
	})
}

type Client struct {
	cc *grpc.ClientConn
}

func NewClient(cc *grpc.ClientConn) *Client {
	return &Client{cc: cc}
}

func (c *Client) Register(ctx context.Context, req *contracts.RegisterRequest) (*contracts.BaseResponse, error) {
	out := new(contracts.BaseResponse)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/Register", req, out)
	return out, err
}

func (c *Client) Login(ctx context.Context, req *contracts.LoginRequest) (*contracts.LoginResponse, error) {
	out := new(contracts.LoginResponse)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/Login", req, out)
	return out, err
}

func (c *Client) ValidateToken(ctx context.Context, req *contracts.ValidateTokenRequest) (*contracts.ValidateTokenResponse, error) {
	out := new(contracts.ValidateTokenResponse)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/ValidateToken", req, out)
	return out, err
}

func (c *Client) GetMe(ctx context.Context, req *contracts.UserIDRequest) (*contracts.MeResponse, error) {
	out := new(contracts.MeResponse)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/GetMe", req, out)
	return out, err
}

func (c *Client) GetPreference(ctx context.Context, req *contracts.UserIDRequest) (*contracts.PreferenceResponse, error) {
	out := new(contracts.PreferenceResponse)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/GetPreference", req, out)
	return out, err
}

func (c *Client) UpdatePreference(ctx context.Context, req *contracts.UpdatePreferenceRequest) (*contracts.BaseResponse, error) {
	out := new(contracts.BaseResponse)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/UpdatePreference", req, out)
	return out, err
}

func (c *Client) GetLocationPermission(ctx context.Context, req *contracts.UserIDRequest) (*contracts.LocationPermissionResponse, error) {
	out := new(contracts.LocationPermissionResponse)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/GetLocationPermission", req, out)
	return out, err
}

func (c *Client) UpdateLocationPermission(ctx context.Context, req *contracts.UpdateLocationPermissionRequest) (*contracts.BaseResponse, error) {
	out := new(contracts.BaseResponse)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/UpdateLocationPermission", req, out)
	return out, err
}

func (c *Client) GetRecommendations(ctx context.Context, req *contracts.RecommendRequest) (*contracts.RecommendResponse, error) {
	out := new(contracts.RecommendResponse)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/GetRecommendations", req, out)
	return out, err
}
