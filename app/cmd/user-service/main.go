package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"xiangchisha/internal/distributed/service"
	"xiangchisha/internal/distributed/userrpc"
	"xiangchisha/internal/platform/config"
	"xiangchisha/internal/rpcjson"
)

func main() {
	addr := config.GetEnv("USER_GRPC_ADDR", "0.0.0.0:50051")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s failed: %v", addr, err)
	}
	srv := grpc.NewServer(grpc.ForceServerCodec(encoding.GetCodec(rpcjson.Name)))
	userrpc.Register(srv, service.NewUserService())
	log.Printf("user-service listening at %s", addr)
	log.Fatal(srv.Serve(lis))
}
