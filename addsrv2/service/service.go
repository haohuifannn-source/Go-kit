package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	"github.com/go-kit/log"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	// ErrTwoZeros不能两个零相加
	ErrTwoZeros = errors.New("不能两个0相加")

	// ErrIntOverflow相加的参数越界
	ErrIntOverflow = errors.New("参数越界")

	// ErrTwonil不能两个空数组相加
	ErrTwonil = errors.New("不能两个空数组相加")

	// ErrEmptyString 两个参数都是空字符串
	ErrEmptyString = errors.New("两个参数都是空字符串")
)

type AddService interface {
	Sum(ctx context.Context, a, b int) (int, error)
	Concat(ctx context.Context, a, b string) (string, error)
}

type addService struct{}

func (s *addService) Sum(ctx context.Context, a, b int) (int, error) {
	return a + b, nil
}

func (s *addService) Concat(ctx context.Context, a, b string) (string, error) {
	if a == "" && b == "" {
		return "", ErrEmptyString
	}
	return a + b, nil
}

func NewService() AddService {
	return &addService{}
}

type logMiddleware struct {
	logger log.Logger
	next   AddService
}

func (mw logMiddleware) Sum(ctx context.Context, a, b int) (res int, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "sum",
			"a", a,
			"b", b,
			"output", res,
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	res, err = mw.next.Sum(ctx, a, b)
	return
}

func (mw logMiddleware) Concat(ctx context.Context, a, b string) (res string, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "sum",
			"a", a,
			"b", b,
			"output", res,
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	res, err = mw.next.Concat(ctx, a, b)
	return
}

// NewLogMiddleware 创建一个带日志的add service
func NewLogMiddleware(logger log.Logger, svc AddService) AddService {
	return &logMiddleware{
		logger: logger,
		next:   svc,
	}
}

// metric
type instrumentingMiddleware struct {
	requestCount   metrics.Counter
	requestLatency metrics.Histogram
	countResult    metrics.Histogram
	next           AddService
}

func (mw instrumentingMiddleware) Sum(ctx context.Context, a, b int) (res int, err error) {
	defer func(begin time.Time) {
		lvs := []string{"method", "sum", "error", fmt.Sprint(err != nil)}
		mw.requestCount.With(lvs...).Add(1)
		mw.requestLatency.With(lvs...).Observe(time.Since(begin).Seconds())
		mw.countResult.Observe(float64(res))
	}(time.Now())

	res, err = mw.next.Sum(ctx, a, b)
	return
}

func (mw instrumentingMiddleware) Concat(ctx context.Context, a, b string) (res string, err error) {
	defer func(begin time.Time) {
		lvs := []string{"method", "concat", "error", "false"}
		mw.requestCount.With(lvs...).Add(1)
		mw.requestLatency.With(lvs...).Observe(time.Since(begin).Seconds())
	}(time.Now())

	res, err = mw.next.Concat(ctx, a, b)
	return
}

func NewInstrumentingMiddleware(svc AddService) AddService {
	fieldKeys := []string{"method", "error"}
	requestCount := kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "request_count",
		Help:      "Number of requests received.",
	}, fieldKeys)
	requestLatency := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "request_latency_microseconds",
		Help:      "Total duration of requests in microseconds.",
	}, fieldKeys)
	countResult := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "count_result",
		Help:      "The result of each count method.",
	}, []string{}) // no fields here
	return &instrumentingMiddleware{
		requestCount:   requestCount,
		requestLatency: requestLatency,
		countResult:    countResult,
		next:           svc,
	}
}
