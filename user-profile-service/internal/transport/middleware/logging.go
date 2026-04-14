package middleware

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC UnaryServerInterceptor that adds structured logging.
func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		traceID := extractTraceID(ctx)

		fields := logrus.Fields{
			"service":   serviceName,
			"method":    info.FullMethod,
			"trace_id":  traceID,
			"peer_addr": extractPeerAddr(ctx),
		}

		entry := logrus.WithFields(fields)
		entry.Info("gRPC request started")

		// Attach trace ID to outgoing context
		md := metadata.New(map[string]string{"x-trace-id": traceID})
		ctx = metadata.NewOutgoingContext(ctx, md)

		resp, err := handler(ctx, req)

		st := status.Convert(err)
		latency := time.Since(start)

		fields["code"] = st.Code().String()
		fields["latency_ms"] = latency.Milliseconds()

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
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if vals := md.Get("x-trace-id"); len(vals) > 0 {
			return vals[0]
		}
	}
	// Generate simple trace ID
	return time.Now().Format("20060102150405") + "-" + randomHex(8)
}

func extractPeerAddr(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
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
