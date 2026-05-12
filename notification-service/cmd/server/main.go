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

	"github.com/dating-bot/notification-service/internal/client"
	"github.com/dating-bot/notification-service/internal/config"
	postgresrepo "github.com/dating-bot/notification-service/internal/repository/postgres"
	"github.com/dating-bot/notification-service/internal/repository/rabbitmq"
	"github.com/dating-bot/notification-service/internal/service"
	transporthttp "github.com/dating-bot/notification-service/internal/transport/http"
)

func main() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
	logrus.WithField("service", "notification-service").Info("starting service")

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

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		logrus.WithError(err).Fatal("failed to ping postgres")
	}
	logrus.Info("postgres connected")

	// ── RabbitMQ subscriber ───────────────────────────────────────────────────
	sub, err := rabbitmq.NewSubscriber(cfg.RabbitMQ.URL)
	if err != nil {
		logrus.WithError(err).Fatal("failed to create rabbitmq subscriber")
	}
	defer sub.Close()

	// ── gRPC client: user-profile-service ─────────────────────────────────────
	upClient, err := client.NewUserProfileClient(cfg.Clients.UserProfileAddr)
	if err != nil {
		logrus.WithError(err).Warn("user-profile-service gRPC not available — notifications will be saved but not delivered")
	}
	defer upClient.Close()

	// ── HTTP clients ──────────────────────────────────────────────────────────
	chatClient := client.NewChatClient(cfg.Clients.ChatServiceURL)
	gwClient := client.NewGatewayClient(cfg.Clients.GatewayURL)

	// ── Repository & Service ──────────────────────────────────────────────────
	notifRepo := postgresrepo.NewNotificationRepository(db)
	notifSvc := service.NewNotificationService(notifRepo, upClient, chatClient, gwClient)

	// ── RabbitMQ subscriptions ────────────────────────────────────────────────
	if err := sub.Subscribe(
		"notification-service.events",
		cfg.RabbitMQ.Exchange,
		notifSvc.HandleEvent,
	); err != nil {
		logrus.WithError(err).Fatal("failed to subscribe to dating.events")
	}
	logrus.Info("subscribed to dating.events")

	// ── HTTP server ───────────────────────────────────────────────────────────
	handler := transporthttp.NewHandler(notifSvc)
	router := transporthttp.NewRouter(handler, cfg.Service.Name)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.HTTP.Port))
	if err != nil {
		logrus.WithError(err).Fatal("failed to listen on HTTP port")
	}

	httpServer := &http.Server{
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logrus.WithField("port", cfg.HTTP.Port).Info("HTTP server started")
		if err := httpServer.Serve(lis); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("HTTP server failed")
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logrus.WithField("signal", sig.String()).Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Warn("HTTP server forced shutdown")
	}

	logrus.Info("notification-service stopped cleanly")
}
