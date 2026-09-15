package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"xiangchisha/internal/distributed/apprpc"
	"xiangchisha/internal/distributed/recommendrpc"
	"xiangchisha/internal/distributed/service"
	"xiangchisha/internal/distributed/userrpc"
	"xiangchisha/internal/platform/config"
	"xiangchisha/internal/rpcjson"
)

func main() {
	userAddr := config.GetEnv("USER_GRPC_ADDR", "127.0.0.1:50051")
	recommendAddr := config.GetEnv("RECOMMEND_GRPC_ADDR", "127.0.0.1:50053")
	appAddr := config.GetEnv("APP_GRPC_ADDR", "0.0.0.0:50050")

	userConn, err := grpc.Dial(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.CallContentSubtype(rpcjson.Name)))
	if err != nil {
		log.Fatalf("dial user-service failed: %v", err)
	}
	defer userConn.Close()
	recommendConn, err := grpc.Dial(recommendAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.CallContentSubtype(rpcjson.Name)))
	if err != nil {
		log.Fatalf("dial recommend-service failed: %v", err)
	}
	defer recommendConn.Close()

	orchestrator := service.NewOrchestratorService(
		userrpc.NewClient(userConn),
		recommendrpc.NewClient(recommendConn),
	)
	lis, err := net.Listen("tcp", appAddr)
	if err != nil {
		log.Fatalf("listen %s failed: %v", appAddr, err)
	}
	srv := grpc.NewServer(grpc.ForceServerCodec(encoding.GetCodec(rpcjson.Name)))
	apprpc.Register(srv, orchestrator)
	log.Printf("app-orchestrator listening at %s", appAddr)
	log.Fatal(srv.Serve(lis))
}
