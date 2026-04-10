package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"trimservice/pb"

	apiconsul "github.com/hashicorp/consul/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const serverName = "trimservice"

type server struct {
	pb.UnimplementedTrimServer
}

var (
	port      = flag.Int("port", 8975, "service port")
	cosulAddr = flag.String("consul", "localhost:8500", "consul address")
)

// TrimSpace 去除字符串参数中的空格
func (s *server) Trim(_ context.Context, req *pb.TrimRequest) (*pb.TrimResponse, error) {
	ov := req.GetStr()
	v := strings.ReplaceAll(ov, " ", "")
	fmt.Printf("ov:%s v:%v\n", ov, v)
	return &pb.TrimResponse{Str: v}, nil
}

func (s *server) Trim2(_ context.Context, req *pb.TrimRequest2) (*pb.TrimResponse2, error) {
	var results []string
	input := req.GetStr()
	for _, str := range input {
		results = append(results, strings.ReplaceAll(str, " ", ""))
	}
	return &pb.TrimResponse2{Str: results}, nil
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", *port))
	if err != nil {
		fmt.Printf("net.Listen %v failed : %v", *port, err)
		return
	}
	s := grpc.NewServer()
	healthcheck := health.NewServer()
	healthpb.RegisterHealthServer(s, healthcheck)
	pb.RegisterTrimServer(s, &server{})

	//服务注册
	cc, err := NewConsulClient(*cosulAddr)
	if err != nil {
		fmt.Printf("NewConsulClient failed : %v\n", err)
		return
	}
	ipInfo, err := getOutboundIP()
	if err != nil {
		fmt.Printf("getOutboundIP failed : %v\n", err)
		return
	}
	if err := cc.RegisterService(serverName, ipInfo.String(), *port); err != nil {
		fmt.Printf("RegisterAddServer failed : %v\n", err)
		return
	}

	go func() {
		err = s.Serve(lis)
		if err != nil {
			fmt.Printf("failed to server : %v", err)
			return
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	cc.Deregister(fmt.Sprintf("%s-%s-%d", serverName, ipInfo.String(), *port))

}

// consul reg&de
type consulClient struct {
	client *apiconsul.Client
}

// NewConsulClient 新建consulClient
func NewConsulClient(consulAddr string) (*consulClient, error) {
	cfg := apiconsul.DefaultConfig()
	cfg.Address = consulAddr
	client, err := apiconsul.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &consulClient{client}, nil
}

// getOutboundIP 获取本机的出口IP
func getOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

// RegisterService 服务注册
func (c *consulClient) RegisterService(serviceName, ip string, port int) error {
	// 健康检查
	check := &apiconsul.AgentServiceCheck{
		GRPC:     fmt.Sprintf("%s:%d", ip, port), // 这里一定是外部可以访问的地址
		Timeout:  "10s",                          // 超时时间
		Interval: "10s",                          // 运行检查的频率
		// 指定时间后自动注销不健康的服务节点
		// 最小超时时间为1分钟，收获不健康服务的进程每30秒运行一次，因此触发注销的时间可能略长于配置的超时时间。
		DeregisterCriticalServiceAfter: "1m",
	}
	srv := &apiconsul.AgentServiceRegistration{
		ID:      fmt.Sprintf("%s-%s-%d", serviceName, ip, port), // 服务唯一ID
		Name:    serviceName,                                    // 服务名称
		Tags:    []string{"q1mi", "trim"},                       // 为服务打标签
		Address: ip,
		Port:    port,
		Check:   check,
	}
	return c.client.Agent().ServiceRegister(srv)
}

// Deregister 注销服务
func (c *consulClient) Deregister(serviceID string) error {
	return c.client.Agent().ServiceDeregister(serviceID)
}
