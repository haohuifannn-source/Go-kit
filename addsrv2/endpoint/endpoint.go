package endpoint

import (
	"addsrv/pb"
	"addsrv/service"
	"context"
	"io"
	"time"

	"github.com/go-kit/log"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/sd"
	"github.com/go-kit/kit/sd/lb"
	grpctransport "github.com/go-kit/kit/transport/grpc"
	"google.golang.org/grpc"

	sdconsul "github.com/go-kit/kit/sd/consul"
	consulapi "github.com/hashicorp/consul/api"
)

func MakeSumEndpoint(s service.AddService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(SumRequest)
		v, err := s.Sum(ctx, req.A, req.B)
		if err != nil {
			return SumResponse{V: v, Err: err.Error()}, nil
		}
		return SumResponse{V: v}, nil
	}
}

func MakeConcatEndpoint(s service.AddService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(ConcatRequest)
		v, err := s.Concat(ctx, req.A, req.B)
		if err != nil {
			return ContatResponse{V: v, Err: err.Error()}, nil
		}
		return ContatResponse{V: v}, nil
	}
}

func MakeTrimEndpoint(conn *grpc.ClientConn) endpoint.Endpoint {
	return grpctransport.NewClient(
		conn,
		"pb.Trim",
		"Trim",
		encodeTrimRequest,
		decodeTrimResponse,
		pb.TrimResponse{},
	).Endpoint()
}

//如果需要实现多个参数传入的时候，需要调用该方法，同时需要配合把concat实现更改
// func MakeTrimEndpoint2(conn *grpc.ClientConn) endpoint.Endpoint {
// 	return grpctransport.NewClient(
// 		conn,
// 		"pb.Trim",
// 		"Trim2",
// 		encodeTrimRequest2,
// 		decodeTrimResponse2,
// 		pb.TrimResponse2{},
// 	).Endpoint()
// }

type SumRequest struct {
	A int `json:"a"`
	B int `json:"b"`
}

type SumResponse struct {
	V   int    `json:"v"`
	Err string `json:"err,omitempty"`
}

type ConcatRequest struct {
	A string `json:"a"`
	B string `json:"b"`
}

type ContatResponse struct {
	V   string `json:"v"`
	Err string `json:"err,omitempty"`
}

type TrimRequest struct {
	S string
}

type TrimResponse struct {
	S string
}

type TrimRequest2 struct {
	S []string
}

type TrimResponse2 struct {
	S []string
}

// Trim服务
type withTrimMiddleware struct {
	trimService endpoint.Endpoint
	next        service.AddService
}

func (mw *withTrimMiddleware) Sum(ctx context.Context, a, b int) (res int, err error) {
	return mw.next.Sum(ctx, a, b)
}

func (mw *withTrimMiddleware) Concat(ctx context.Context, a, b string) (res string, err error) {
	//首先调用Trim服务去掉可能存在的空格
	respA, err := mw.trimService(ctx, TrimRequest{S: a})
	if err != nil {
		return "", err
	}
	respB, err := mw.trimService(ctx, TrimRequest{S: b})
	if err != nil {
		return "", err
	}

	trimA := respA.(*TrimResponse)
	trimB := respB.(*TrimResponse)
	//然后调用自己的服务去拼接字符
	return mw.next.Concat(ctx, trimA.S, trimB.S)
}

// Concat2可以传入多个参数进行处理
// func (mw *withTrimMiddleware) Concat2(ctx context.Context, a, b string) (res string, err error) {
// 	//首先调用Trim服务去掉可能存在的空格
// 	req := TrimRequest2{S: []string{a, b}}
// 	resp, err := mw.trimService(ctx, req)
// 	if err != nil {
// 		return "", err
// 	}
// 	trimResp := resp.(*TrimResponse2)
// 	//然后调用自己的服务去拼接字符
// 	return mw.next.Concat(ctx, trimResp.S[0], trimResp.S[1])
// }

func NewTrimMiddleware(trimEndpoint endpoint.Endpoint, svc service.AddService) service.AddService {
	return &withTrimMiddleware{
		trimService: trimEndpoint,
		next:        svc,
	}
}

// Trim的编解码----注意这里是反过来，因为是作为客户端进行传输
func encodeTrimRequest(_ context.Context, request interface{}) (interface{}, error) {
	req := request.(TrimRequest)
	return &pb.TrimRequest{Str: req.S}, nil
}

func decodeTrimResponse(_ context.Context, response interface{}) (interface{}, error) {
	rep := response.(*pb.TrimResponse)
	return &TrimResponse{S: rep.Str}, nil
}

// Trim的编解码2--多个参数处理
// func encodeTrimRequest2(_ context.Context, request interface{}) (interface{}, error) {
// 	req := request.(TrimRequest2)
// 	return &pb.TrimRequest2{Str: req.S}, nil
// }

// func decodeTrimResponse2(_ context.Context, response interface{}) (interface{}, error) {
// 	rep := response.(*pb.TrimResponse2)
// 	return &TrimResponse2{S: rep.Str}, nil
// }

// getTrimServiceFromConsul实现基于consul的服务发现，无需自己去指定服务端口
func GetTrimServiceFromConsul(consulAddr string, svname string, tags []string, logger log.Logger) (endpoint.Endpoint, error) {
	//1、链接consul
	consulcfg := consulapi.DefaultConfig()
	consulcfg.Address = consulAddr

	cc, err := consulapi.NewClient(consulcfg)
	if err != nil {
		return nil, err
	}

	//2、使用go-kit提供的适配器
	sdClient := sdconsul.NewClient(cc)
	instancer := sdconsul.NewInstancer(sdClient, logger, svname, tags, true)

	//3、Endpoint
	endpointer := sd.NewEndpointer(instancer, factory, logger)

	//4、Balancer
	balancer := lb.NewRoundRobin(endpointer)

	//5、Retry
	retry := lb.Retry(3, time.Second, balancer)

	return retry, nil

}

func factory(instance string) (endpoint.Endpoint, io.Closer, error) {
	conn, err := grpc.Dial(instance, grpc.WithInsecure())
	if err != nil {
		return nil, nil, err
	}

	e := MakeTrimEndpoint(conn)
	return e, conn, err
}
