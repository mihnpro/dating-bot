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

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	recommendationv1 "github.com/mihnpro/DatingBotProtos/gen/go/recommendation/v1"

	"github.com/dating-bot/recommendation-service/internal/cache"
	"github.com/dating-bot/recommendation-service/internal/client"
	"github.com/dating-bot/recommendation-service/internal/config"
	"github.com/dating-bot/recommendation-service/internal/domain/entity"
	postgresrepo "github.com/dating-bot/recommendation-service/internal/repository/postgres"
	"github.com/dating-bot/recommendation-service/internal/repository/rabbitmq"
	"github.com/dating-bot/recommendation-service/internal/service"
	"github.com/dating-bot/recommendation-service/internal/service/worker"
	grpcserver "github.com/dating-bot/recommendation-service/internal/transport/grpc"
	"github.com/dating-bot/recommendation-service/internal/transport/metrics"
	"github.com/dating-bot/recommendation-service/internal/transport/middleware"
)

func main() {
	// ── Logging ───────────────────────────────────────────────────────────────
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
	logrus.WithField("service", "recommendation-service").Info("Starting service")

	// ── Config ────────────────────────────────────────────────────────────────
	cfg, err := config.Load("")
	if err != nil {
		logrus.WithError(err).Fatal("failed to load config")
	}

	// ── PostgreSQL ────────────────────────────────────────────────────────────
	db, err := sql.Open("postgres", cfg.Postgres.DSN())
	if err != nil {
		logrus.WithError(err).Fatal("failed to open postgres")
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		logrus.WithError(err).Fatal("failed to ping postgres")
	}
	logrus.Info("postgres connected")

	// ── Redis ─────────────────────────────────────────────────────────────────
	redisCache := cache.NewRecommendationCache(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.Redis.CacheTTLSeconds,
	)
	defer redisCache.Close()

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := redisCache.Ping(pingCtx); err != nil {
		logrus.WithError(err).Fatal("failed to ping redis")
	}
	logrus.Info("redis connected")

	// ── RabbitMQ publisher ────────────────────────────────────────────────────
	pub, err := rabbitmq.NewPublisher(cfg.RabbitMQ.URL, cfg.RabbitMQ.Exchange)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create rabbitmq publisher")
	}
	defer pub.Close()

	// ── RabbitMQ subscriber ───────────────────────────────────────────────────
	sub, err := rabbitmq.NewSubscriber(cfg.RabbitMQ.URL)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create rabbitmq subscriber")
	}
	defer sub.Close()

	// ── gRPC clients ──────────────────────────────────────────────────────────
	if err := client.InitUserProfileClient(cfg.Clients.UserProfileAddr); err != nil {
		logrus.WithError(err).Warn("user-profile-service not available — primary ratings disabled")
	}
	upClient := client.GetUserProfileClient()
	defer upClient.Close()

	if err := client.InitMatching(cfg.Clients.MatchingAddr); err != nil {
		logrus.WithError(err).Warn("matching-service not available — behavioral ratings disabled")
	}
	matchClient := client.GetMatching()
	defer matchClient.Close()

	// ── Repository ────────────────────────────────────────────────────────────
	ratingRepo := postgresrepo.NewRatingRepository(db)

	// ── Worker pool ───────────────────────────────────────────────────────────
	// The handler is registered by NewRecommendationService below via SetHandler.
	pool := worker.New(cfg.Worker.PoolSize, cfg.Worker.QueueSize, nil)

	// ── Domain service ────────────────────────────────────────────────────────
	svc := service.NewRecommendationService(
		ratingRepo,
		redisCache,
		upClient,
		matchClient,
		pub,
		pool,
		50, // batch size
	)

	// ── Start the worker pool ─────────────────────────────────────────────────
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	pool.Start(appCtx)

	// ── Periodic global recalculation ticker ──────────────────────────────────
	go func() {
		interval := time.Duration(cfg.Worker.RecalcIntervalMinutes) * time.Minute
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		logrus.WithField("interval_minutes", cfg.Worker.RecalcIntervalMinutes).
			Info("periodic recalculation ticker started")

		for {
			select {
			case <-ticker.C:
				logrus.Info("periodic global recalculation triggered")
				pool.Enqueue(worker.Job{Type: worker.JobGlobalRecalc})

			case <-appCtx.Done():
				logrus.Info("periodic recalculation ticker stopped")
				return
			}
		}
	}()

	// ── RabbitMQ event subscriptions ─────────────────────────────────────────
	if err := sub.Subscribe(
		"recommendation-service.profile.updated",
		cfg.RabbitMQ.Exchange,
		func(evt *entity.DomainEvent) error {
			if evt.EventName != "profile.updated" {
				return nil // not our event — ack and move on
			}
			return svc.HandleProfileUpdated(evt)
		},
	); err != nil {
		logrus.WithError(err).Fatal("failed to subscribe to profile.updated")
	}
	logrus.Info("subscribed to profile.updated")

	if err := sub.Subscribe(
		"recommendation-service.interaction.liked",
		cfg.RabbitMQ.Exchange,
		func(evt *entity.DomainEvent) error {
			if evt.EventName != "interaction.liked" {
				return nil
			}
			return svc.HandleInteractionLiked(evt)
		},
	); err != nil {
		logrus.WithError(err).Fatal("failed to subscribe to interaction.liked")
	}
	logrus.Info("subscribed to interaction.liked")

	// ── gRPC server ───────────────────────────────────────────────────────────
	grpcSrv := grpcserver.NewServer(svc)

	grpcOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			metrics.UnaryServerInterceptor(),
			middleware.UnaryServerInterceptor(cfg.Service.Name),
		),
	}

	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	if err != nil {
		logrus.WithError(err).Fatal("failed to listen on gRPC port")
	}

	grpcServer := grpc.NewServer(grpcOpts...)
	recommendationv1.RegisterRecommendationServiceServer(grpcServer, grpcSrv)
	reflection.Register(grpcServer)

	go func() {
		logrus.WithField("port", cfg.GRPC.Port).Info("gRPC server started")
		if err := grpcServer.Serve(grpcLis); err != nil {
			logrus.WithError(err).Fatal("gRPC server failed")
		}
	}()

	// ── HTTP gateway + /metrics + /health ─────────────────────────────────────
	httpLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.HTTP.Port))
	if err != nil {
		logrus.WithError(err).Fatal("failed to listen on HTTP port")
	}

	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Worker pool stats endpoint — useful for debugging.
	mux.HandleFunc("/debug/pool", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w,
			`{"queue_len":%d,"processed":%d,"dropped":%d}`,
			pool.QueueLen(),
			pool.Processed(),
			pool.Dropped(),
		)
	})

	gwmux := runtime.NewServeMux()
	gwOpts := []grpc.DialOption{grpc.WithInsecure()}
	grpcAddr := fmt.Sprintf("localhost:%d", cfg.GRPC.Port)

	if err := recommendationv1.RegisterRecommendationServiceHandlerFromEndpoint(
		context.Background(), gwmux, grpcAddr, gwOpts,
	); err != nil {
		logrus.WithError(err).Fatal("failed to register grpc-gateway")
	}

	mux.Handle("/", middleware.HTTPMiddleware(cfg.Service.Name)(gwmux))

	httpServer := &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logrus.WithField("port", cfg.HTTP.Port).Info("HTTP gateway started")
		if err := httpServer.Serve(httpLis); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("HTTP server failed")
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logrus.WithField("signal", sig.String()).Info("shutdown signal received")

	// Cancel the app context — stops the ticker and signals the pool.
	appCancel()

	// Stop accepting new gRPC requests and finish in-flight ones.
	grpcServer.GracefulStop()
	logrus.Info("gRPC server stopped")

	// Give the HTTP server a short window to drain.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Warn("HTTP server forced shutdown")
	}
	logrus.Info("HTTP server stopped")

	// Wait for all worker jobs to finish.
	pool.Stop()
	logrus.Info("worker pool drained")

	logrus.Info("recommendation-service stopped cleanly")
}
