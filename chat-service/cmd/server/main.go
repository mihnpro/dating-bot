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

	chatv1 "github.com/mihnpro/DatingBotProtos/gen/go/chat/v1"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/dating-bot/chat-service/internal/client"
	"github.com/dating-bot/chat-service/internal/config"
	postgresrepo "github.com/dating-bot/chat-service/internal/repository/postgres"
	"github.com/dating-bot/chat-service/internal/repository/rabbitmq"
	"github.com/dating-bot/chat-service/internal/service"
	httphandler "github.com/dating-bot/chat-service/internal/transport/http"
	grpcserver "github.com/dating-bot/chat-service/internal/transport/grpc"
	"github.com/dating-bot/chat-service/internal/transport/middleware"
	wshandler "github.com/dating-bot/chat-service/internal/transport/websocket"
)

func main() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
	logrus.WithField("service", "chat-service").Info("Starting service")

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

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		logrus.WithError(err).Fatal("Failed to ping database")
	}

	// --- gRPC client: user-profile-service ---
	upClient, err := client.NewUserProfileClient(cfg.Clients.UserProfileServiceAddr)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to user-profile-service")
	}
	defer upClient.Close()

	// --- RabbitMQ ---
	pub, err := rabbitmq.NewPublisher(cfg.RabbitMQ.URL, "dating.events")
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect RabbitMQ publisher")
	}
	defer pub.Close()

	sub, err := rabbitmq.NewSubscriber(cfg.RabbitMQ.URL)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect RabbitMQ subscriber")
	}
	defer sub.Close()

	// --- Repositories ---
	convRepo := postgresrepo.NewConversationRepository(db)
	msgRepo := postgresrepo.NewMessageRepository(db)

	// --- WebSocket Hub ---
	hub := wshandler.NewHub()
	go hub.Run()

	// --- Services ---
	tokenSvc := service.NewTokenService(cfg.Chat.SecretKey, time.Duration(cfg.Chat.TokenTTLSec)*time.Second)
	chatSvc := service.NewChatService(convRepo, msgRepo, pub, hub, upClient)

	// Subscribe to match.created → auto-create conversation
	if err := sub.Subscribe("dating.events", "chat-service.match.created", chatSvc.OnMatchCreated); err != nil {
		logrus.WithError(err).Fatal("Failed to subscribe to match.created")
	}

	// --- gRPC Server ---
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	if err != nil {
		logrus.WithError(err).Fatal("Failed to listen on gRPC port")
	}

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcserver.UnaryLoggingInterceptor(cfg.Service.Name),
		),
	)
	chatv1.RegisterChatServiceServer(grpcSrv, grpcserver.NewServer(chatSvc, tokenSvc))
	reflection.Register(grpcSrv)

	go func() {
		logrus.WithField("port", cfg.GRPC.Port).Info("gRPC server started")
		if err := grpcSrv.Serve(grpcLis); err != nil {
			logrus.WithError(err).Fatal("gRPC server error")
		}
	}()

	// --- HTTP + WebSocket Server ---
	handler := httphandler.NewHandler(chatSvc, tokenSvc, hub)
	router := httphandler.NewRouter(handler)
	loggedRouter := middleware.Logging(cfg.Service.Name)(router)

	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:      loggedRouter,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logrus.WithField("port", cfg.HTTP.Port).Info("HTTP server started")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("HTTP server error")
		}
	}()

	// --- Graceful Shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down...")
	grpcSrv.GracefulStop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	logrus.Info("Server stopped")
}
