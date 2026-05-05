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
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	mediav1 "github.com/mihnpro/DatingBotProtos/gen/go/media/v1"

	"github.com/dating-bot/media-service/internal/config"
	"github.com/dating-bot/media-service/internal/repository/postgres"
	"github.com/dating-bot/media-service/internal/repository/rabbitmq"
	s3repo "github.com/dating-bot/media-service/internal/repository/s3"
	"github.com/dating-bot/media-service/internal/service"
	grpctransport "github.com/dating-bot/media-service/internal/transport/grpc"
	transporthttp "github.com/dating-bot/media-service/internal/transport/http"
	"github.com/dating-bot/media-service/internal/transport/metrics"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.Load("")
	if err != nil {
		log.WithError(err).Fatal("failed to load config")
	}

	log.WithField("service", cfg.Service.Name).Info("starting")

	db, err := connectPostgres(cfg.Postgres.DSN())
	if err != nil {
		log.WithError(err).Fatal("failed to connect to postgres")
	}
	defer db.Close()

	ctx := context.Background()

	storage, err := s3repo.NewMinioStorage(ctx, s3repo.Config{
		Endpoint:   cfg.Minio.Endpoint,
		AccessKey:  cfg.Minio.AccessKey,
		SecretKey:  cfg.Minio.SecretKey,
		Bucket:     cfg.Minio.Bucket,
		UseSSL:     cfg.Minio.UseSSL,
		PublicHost: cfg.Minio.PublicHost,
	})
	if err != nil {
		log.WithError(err).Fatal("failed to connect to minio")
	}

	publisher, err := rabbitmq.NewPublisher(cfg.RabbitMQ.URL, cfg.RabbitMQ.Exchange)
	if err != nil {
		log.WithError(err).Fatal("failed to connect to rabbitmq")
	}
	defer publisher.Close()

	mediaRepo := postgres.NewMediaRepository(db)
	m := metrics.New(cfg.Service.Name)
	svc := service.NewMediaService(mediaRepo, storage, publisher, log)

	// gRPC server
	grpcSrv := grpc.NewServer()
	mediav1.RegisterMediaServiceServer(grpcSrv, grpctransport.NewServer(svc))
	reflection.Register(grpcSrv)

	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	if err != nil {
		log.WithError(err).Fatal("failed to listen grpc")
	}

	go func() {
		log.WithField("port", cfg.GRPC.Port).Info("grpc server listening")
		if err := grpcSrv.Serve(grpcLis); err != nil {
			log.WithError(err).Fatal("grpc server error")
		}
	}()

	// HTTP server (multipart upload + grpc-gateway for other endpoints)
	httpHandler := transporthttp.NewHandler(svc, m, log)
	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:      httpHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.WithField("port", cfg.HTTP.Port).Info("http server listening")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("http server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	grpcSrv.GracefulStop()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Error("http graceful shutdown failed")
	}

	log.Info("stopped")
}

func connectPostgres(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	return db, nil
}
