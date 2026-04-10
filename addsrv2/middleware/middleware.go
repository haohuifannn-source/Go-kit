package middleware

import (
	"context"
	"errors"
	"time"

	"github.com/go-kit/log"

	"github.com/go-kit/kit/endpoint"
	"golang.org/x/time/rate"
)

var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// LoggingMiddleware日志中间件
func LoggingMiddleware(logger log.Logger) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			logger.Log("msg", "calling endpoint")
			start := time.Now()
			defer logger.Log("msg", "called endpoint", "took", time.Since(start))
			return next(ctx, request)
		}
	}
}

// rateMiddleware限流中间件
func RateMiddleware(limit *rate.Limiter) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			if !limit.Allow() {
				return nil, ErrRateLimitExceeded
			}
			return next(ctx, request)
		}
	}
}
