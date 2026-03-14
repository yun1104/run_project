package recommendrpc

import (
	"context"
	"google.golang.org/grpc"

	"xiangchisha/internal/distributed/contracts"
	_ "xiangchisha/internal/rpcjson"
)

const ServiceName = "recommend.RecommendService"

type Service interface {
	GetRecommendations(context.Context, *contracts.RecommendRequest) (*contracts.RecommendResponse, error)
}

func Register(server *grpc.Server, impl Service) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: ServiceName,
		HandlerType: (*Service)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "GetRecommendations", Handler: wrapGetRecommendations(impl)},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "proto/recommend.proto",
	}, impl)
}

func wrapGetRecommendations(impl Service) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		in := new(contracts.RecommendRequest)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return impl.GetRecommendations(ctx, in)
		}
		info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ServiceName + "/GetRecommendations"}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return impl.GetRecommendations(ctx, req.(*contracts.RecommendRequest))
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

func (c *Client) GetRecommendations(ctx context.Context, req *contracts.RecommendRequest) (*contracts.RecommendResponse, error) {
	out := new(contracts.RecommendResponse)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/GetRecommendations", req, out)
	return out, err
}
