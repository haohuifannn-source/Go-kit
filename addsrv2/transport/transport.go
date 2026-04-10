package transport

import (
	"addsrv/endpoint"
	"addsrv/middleware"
	"addsrv/pb"
	"addsrv/service"
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	grpctransport "github.com/go-kit/kit/transport/grpc"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/go-kit/log"
	"golang.org/x/time/rate"
)

type server struct {
	pb.UnimplementedAddServer
	sum    grpctransport.Handler
	concat grpctransport.Handler
}

func (s *server) Sum(ctx context.Context, in *pb.SumRequest) (*pb.SumResponse, error) {
	_, resp, err := s.sum.ServeGRPC(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.SumResponse), nil
}

func (s *server) Concat(ctx context.Context, in *pb.ConcatRequest) (*pb.ConcatResponse, error) {
	_, resp, err := s.concat.ServeGRPC(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.ConcatResponse), nil
}

// decodeGRPCSumRequest 将Sum方法的gRPC请求参数转为内部的SumRequest
func decodeGRPCSumRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*pb.SumRequest)
	return endpoint.SumRequest{A: int(req.A), B: int(req.B)}, nil
}

// decodeGRPCConcatRequest 将Concat方法的gRPC请求参数转为内部的ConcatRequest
func decodeGRPCConcatRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*pb.ConcatRequest)
	return endpoint.ConcatRequest{A: req.A, B: req.B}, nil
}

// encodeGRPCSumResponse 封装Sum的gRPC响应
func encodeGRPCSumResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(endpoint.SumResponse)
	return &pb.SumResponse{Val: int64(resp.V), Err: resp.Err}, nil
}

// encodeGRPCConcatResponse 封装Concat的gRPC响应
func encodeGRPCConcatResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(endpoint.ContatResponse)
	return &pb.ConcatResponse{Val: resp.V, Err: resp.Err}, nil
}

// NewGRPCServer grpcServer构造函数
func NewGRPCServer(svc service.AddService) pb.AddServer {
	return &server{
		sum: grpctransport.NewServer(
			endpoint.MakeSumEndpoint(svc),
			decodeGRPCSumRequest,
			encodeGRPCSumResponse,
		),
		concat: grpctransport.NewServer(
			endpoint.MakeConcatEndpoint(svc),
			decodeGRPCConcatRequest,
			encodeGRPCConcatResponse,
		),
	}
}

//HTTP的通信方式

func decodeSumRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var request endpoint.SumRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

func decodeConcatRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var request endpoint.ConcatRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

func encodeResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	return json.NewEncoder(w).Encode(response)
}

// NewGRPCServer grpcServer构造函数
func NewHttpServer(svc service.AddService, logger log.Logger) http.Handler {
	sum := endpoint.MakeSumEndpoint(svc)
	//log.With(logger, "method", "sum")派生子logger的效果，使得每次调用都会带上这个标签
	sum = middleware.LoggingMiddleware(log.With(logger, "method", "sum"))(sum)
	//使用限流中间件
	sum = middleware.RateMiddleware(rate.NewLimiter(1, 1))(sum)
	sumHandler := httptransport.NewServer(
		sum, //日志中间件包一层的sum，使得每次调用都会带上这个标签
		decodeSumRequest,
		encodeResponse,
	)
	concatHandler := httptransport.NewServer(
		endpoint.MakeConcatEndpoint(svc),
		decodeConcatRequest,
		encodeResponse,
	)
	// r := mux.NewRouter()
	// r.Handle("/sum", sumHandler).Methods("POST")
	// r.Handle("/concat", concatHandler).Methods("POST")
	//GIN框架
	r := gin.Default()
	r.POST("/sum", gin.WrapH(sumHandler))
	r.POST("/concat", gin.WrapH(concatHandler))
	return r
}
