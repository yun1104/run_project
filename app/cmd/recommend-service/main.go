package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"xiangchisha/internal/distributed/recommendrpc"
	"xiangchisha/internal/distributed/service"
	"xiangchisha/internal/platform/config"
	"xiangchisha/internal/rpcjson"
)

func main() {
	addr := config.GetEnv("RECOMMEND_GRPC_ADDR", "0.0.0.0:50053")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s failed: %v", addr, err)
	}
	srv := grpc.NewServer(grpc.ForceServerCodec(encoding.GetCodec(rpcjson.Name)))
	recommendrpc.Register(srv, service.NewRecommendService())
	log.Printf("recommend-service listening at %s", addr)
	log.Fatal(srv.Serve(lis))
}
