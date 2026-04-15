package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	_ "github.com/lib/pq"

	userprofilev1 "github.com/mihnpro/DatingBotProtos/gen/go/user-profile/v1"

	"github.com/dating-bot/user-profile-service/internal/config"
	"github.com/dating-bot/user-profile-service/internal/service"
	postgresrepo "github.com/dating-bot/user-profile-service/internal/repository/postgres"
	"github.com/dating-bot/user-profile-service/internal/repository/rabbitmq"
	grpcserver "github.com/dating-bot/user-profile-service/internal/transport/grpc"
	"github.com/dating-bot/user-profile-service/internal/transport/metrics"
	"github.com/dating-bot/user-profile-service/internal/transport/middleware"
)

func main() {
	// --- Logging ---
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
	logrus.WithField("service", "user-profile-service").Info("Starting service")

	cfg, err := config.Load("")
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load config")
	}

	// --- PostgreSQL ---
	db, err := sql.Open("postgres", cfg.Postgres.DSN())
	if err != nil {
		logrus.WithError(err).Fatal("Failed to open database")
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logrus.WithError(err).Fatal("Failed to ping database")
	}

	// --- RabbitMQ ---
	pub, err := rabbitmq.NewPublisher(cfg.RabbitMQ.URL, "dating.events")
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to RabbitMQ")
	}
	defer pub.Close()

	// --- Repositories ---
	userRepo := postgresrepo.NewUserRepository(db)
	profileRepo := postgresrepo.NewProfileRepository(db)

	// --- Domain Service ---
	svc := service.NewUserService(userRepo, profileRepo, pub)

	// --- gRPC Server ---
	grpcSrv := grpcserver.NewServer(svc)

	grpcOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			metrics.UnaryServerInterceptor(),                    // Prometheus metrics
			middleware.UnaryServerInterceptor(cfg.Service.Name), // Structured logging
		),
	}

	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	if err != nil {
		logrus.WithError(err).Fatal("Failed to listen on gRPC port")
	}

	grpcServer := grpc.NewServer(grpcOpts...)
	userprofilev1.RegisterUserServiceServer(grpcServer, grpcSrv)
	reflection.Register(grpcServer)

	go func() {
		logrus.WithField("port", cfg.GRPC.Port).Info("gRPC server started")
		if err := grpcServer.Serve(grpcLis); err != nil {
			logrus.WithError(err).Fatal("gRPC server failed")
		}
	}()

	// --- HTTP Gateway + /metrics endpoint ---
	httpLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.HTTP.Port))
	if err != nil {
		logrus.WithError(err).Fatal("Failed to listen on HTTP port")
	}

	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// gRPC-Gateway
	gwmux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	grpcAddr := fmt.Sprintf("localhost:%d", cfg.GRPC.Port)

	if err := userprofilev1.RegisterUserServiceHandlerFromEndpoint(context.Background(), gwmux, grpcAddr, opts); err != nil {
		logrus.WithError(err).Fatal("Failed to register gateway")
	}

	mux.Handle("/", middleware.HTTPMiddleware(cfg.Service.Name)(gwmux))

	httpServer := &http.Server{Handler: mux}

	go func() {
		logrus.WithField("port", cfg.HTTP.Port).Info("HTTP gateway + metrics started")
		if err := httpServer.Serve(httpLis); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("HTTP server failed")
		}
	}()

	// --- Graceful Shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down...")
	grpcServer.GracefulStop()
	httpServer.Close()
	logrus.Info("Server stopped")
}
