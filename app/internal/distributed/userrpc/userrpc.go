package userrpc

import (
	"context"
	"google.golang.org/grpc"

	"xiangchisha/internal/distributed/contracts"
	_ "xiangchisha/internal/rpcjson"
)

const ServiceName = "user.UserService"

type Service interface {
	Register(context.Context, *contracts.RegisterRequest) (*contracts.BaseResponse, error)
	Login(context.Context, *contracts.LoginRequest) (*contracts.LoginResponse, error)
	GetUserInfo(context.Context, *contracts.UserIDRequest) (*contracts.MeResponse, error)
	GetPreference(context.Context, *contracts.UserIDRequest) (*contracts.PreferenceResponse, error)
	UpdatePreference(context.Context, *contracts.UpdatePreferenceRequest) (*contracts.BaseResponse, error)
	GetLocationPermission(context.Context, *contracts.UserIDRequest) (*contracts.LocationPermissionResponse, error)
	UpdateLocationPermission(context.Context, *contracts.UpdateLocationPermissionRequest) (*contracts.BaseResponse, error)
	ValidateToken(context.Context, *contracts.ValidateTokenRequest) (*contracts.ValidateTokenResponse, error)
}

func Register(server *grpc.Server, impl Service) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: ServiceName,
		HandlerType: (*Service)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Register", Handler: wrapRegister(impl)},
			{MethodName: "Login", Handler: wrapLogin(impl)},
			{MethodName: "GetUserInfo", Handler: wrapGetUserInfo(impl)},
			{MethodName: "GetPreference", Handler: wrapGetPreference(impl)},
			{MethodName: "UpdatePreference", Handler: wrapUpdatePreference(impl)},
			{MethodName: "GetLocationPermission", Handler: wrapGetLocationPermission(impl)},
			{MethodName: "UpdateLocationPermission", Handler: wrapUpdateLocationPermission(impl)},
			{MethodName: "ValidateToken", Handler: wrapValidateToken(impl)},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "proto/user.proto",
	}, impl)
}

func wrapRegister(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		in := new(contracts.RegisterRequest)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.Register(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ServiceName + "/Register"}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return impl.Register(ctx, req.(*contracts.RegisterRequest))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func wrapLogin(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		in := new(contracts.LoginRequest)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.Login(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ServiceName + "/Login"}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return impl.Login(ctx, req.(*contracts.LoginRequest))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func wrapGetUserInfo(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		in := new(contracts.UserIDRequest)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.GetUserInfo(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ServiceName + "/GetUserInfo"}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return impl.GetUserInfo(ctx, req.(*contracts.UserIDRequest))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func wrapGetPreference(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		in := new(contracts.UserIDRequest)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.GetPreference(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ServiceName + "/GetPreference"}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return impl.GetPreference(ctx, req.(*contracts.UserIDRequest))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func wrapUpdatePreference(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		in := new(contracts.UpdatePreferenceRequest)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.UpdatePreference(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ServiceName + "/UpdatePreference"}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return impl.UpdatePreference(ctx, req.(*contracts.UpdatePreferenceRequest))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func wrapGetLocationPermission(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		in := new(contracts.UserIDRequest)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.GetLocationPermission(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ServiceName + "/GetLocationPermission"}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return impl.GetLocationPermission(ctx, req.(*contracts.UserIDRequest))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func wrapUpdateLocationPermission(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		in := new(contracts.UpdateLocationPermissionRequest)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.UpdateLocationPermission(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ServiceName + "/UpdateLocationPermission"}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return impl.UpdateLocationPermission(ctx, req.(*contracts.UpdateLocationPermissionRequest))
		}
		return interceptor(ctx, in, info, handler)
	}
}

func wrapValidateToken(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		in := new(contracts.ValidateTokenRequest)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.ValidateToken(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ServiceName + "/ValidateToken"}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return impl.ValidateToken(ctx, req.(*contracts.ValidateTokenRequest))
		}
		return interceptor(ctx, in, info, handler)
	}
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

func (c *Client) GetUserInfo(ctx context.Context, req *contracts.UserIDRequest) (*contracts.MeResponse, error) {
	out := new(contracts.MeResponse)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/GetUserInfo", req, out)
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

func (c *Client) ValidateToken(ctx context.Context, req *contracts.ValidateTokenRequest) (*contracts.ValidateTokenResponse, error) {
	out := new(contracts.ValidateTokenResponse)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/ValidateToken", req, out)
	return out, err
}
