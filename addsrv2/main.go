package main

import (
	"addsrv/endpoint"
	"addsrv/pb"
	"addsrv/service"
	"addsrv/transport"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

var (
	httpAddr = flag.Int("http-addr", 8080, "HTTP端口")
	grpcAddr = flag.Int("grpc-addr", 8972, "HTTP端口")
	trimAddr = flag.String("trim-addr", "127.0.0.1:8975", "trim地址")
)

func main() {
	flag.Parse()
	logger := log.NewJSONLogger(log.NewSyncWriter(os.Stdout))
	src := service.NewService()
	src = service.NewLogMiddleware(logger, src)
	src = service.NewInstrumentingMiddleware(src)

	// conn, err := grpc.Dial(*trimAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	// if err != nil {
	// 	fmt.Printf("connect %s failed, err: %v", *trimAddr, err)
	// 	return
	// }
	// defer conn.Close()
	// trimEndpoint := endpoint.MakeTrimEndpoint(conn)
	trimEndpoint, err := endpoint.GetTrimServiceFromConsul("localhost:8500", "trimservice", nil, logger)
	if err != nil {
		fmt.Printf("GetTrimServiceFromConsul failed : %v", err)
		return
	}
	src = endpoint.NewTrimMiddleware(trimEndpoint, src)

	// trimEndpoint2 := endpoint.MakeTrimEndpoint2(conn)
	// src = endpoint.NewTrimMiddleware(trimEndpoint2, src)

	var g errgroup.Group

	//HTTP
	g.Go(func() error {
		httpListener, err := net.Listen("tcp", fmt.Sprintf(":%d", *httpAddr))
		if err != nil {
			fmt.Printf("http net.Listen %d failed, err : %v\n", *httpAddr, err)
			return err
		}
		defer httpListener.Close()
		logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
		httHandler := transport.NewHttpServer(src, logger)
		httHandler.(*gin.Engine).GET("/metrics", gin.WrapH(promhttp.Handler()))
		return http.Serve(httpListener, httHandler)
	})

	//GRPC
	g.Go(func() error {
		gRPCListener, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpcAddr))
		if err != nil {
			fmt.Printf("grpc net.Listen %d failed, err : %v\n", *grpcAddr, err)
			return err
		}
		s := grpc.NewServer()
		pb.RegisterAddServer(s, transport.NewGRPCServer(src))
		return s.Serve(gRPCListener)
	})

	if err := g.Wait(); err != nil {
		fmt.Println("所有服务已停止，原因是:", err)
	}

}
