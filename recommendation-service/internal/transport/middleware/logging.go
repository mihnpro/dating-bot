package middleware

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC UnaryServerInterceptor that adds
// structured JSON logging for every inbound RPC — latency, status code,
// trace-id and peer address are all captured.
func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()
		traceID := extractTraceID(ctx)

		entry := logrus.WithFields(logrus.Fields{
			"service":   serviceName,
			"method":    info.FullMethod,
			"trace_id":  traceID,
			"peer_addr": extractPeerAddr(ctx),
		})
		entry.Info("gRPC request started")

		// Propagate trace-id downstream.
		md := metadata.New(map[string]string{"x-trace-id": traceID})
		ctx = metadata.NewOutgoingContext(ctx, md)

		resp, err := handler(ctx, req)

		st := status.Convert(err)
		fields := logrus.Fields{
			"code":       st.Code().String(),
			"latency_ms": time.Since(start).Milliseconds(),
		}

		if err != nil {
			fields["error"] = st.Message()
			entry.WithFields(fields).Warn("gRPC request failed")
		} else {
			entry.WithFields(fields).Info("gRPC request completed")
		}

		return resp, err
	}
}

func extractTraceID(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-trace-id"); len(vals) > 0 {
			return vals[0]
		}
	}
	return time.Now().Format("20060102150405") + "-" + randomHex(8)
}

func extractPeerAddr(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(":authority"); len(vals) > 0 {
			return vals[0]
		}
	}
	return "unknown"
}

func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hex[time.Now().UnixNano()%16]
	}
	return string(b)
}
