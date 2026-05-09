package grpc

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryLoggingInterceptor logs every gRPC call with method, status and latency.
func UnaryLoggingInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err).String()
		logrus.WithFields(logrus.Fields{
			"service":    serviceName,
			"method":     info.FullMethod,
			"code":       code,
			"latency_ms": time.Since(start).Milliseconds(),
		}).Info("grpc request")
		return resp, err
	}
}
