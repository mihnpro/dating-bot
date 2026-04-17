package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	GRPC     GRPCConfig
	HTTP     HTTPConfig
	Postgres PostgresConfig
	RabbitMQ RabbitMQConfig
	Redis    RedisConfig
	Service  ServiceConfig
	Clients  ClientsConfig
	Worker   WorkerConfig
}

type GRPCConfig struct {
	Port int
}

type HTTPConfig struct {
	Port int
}

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RabbitMQConfig struct {
	URL      string
	Exchange string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	// TTL for cached recommendation queues (seconds)
	CacheTTLSeconds int
}

type ServiceConfig struct {
	Name string
}

type ClientsConfig struct {
	UserProfileAddr string
	MatchingAddr    string
}

type WorkerConfig struct {
	// Number of goroutines in the rating recalculation pool
	PoolSize int
	// Buffered channel capacity for recalculation jobs
	QueueSize int
	// How often (minutes) a full periodic recalculation is triggered
	RecalcIntervalMinutes int
}

func Load(path string) (*Config, error) {
	if path != "" {
		viper.SetConfigFile(path)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath(".")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	cfg := &Config{
		GRPC: GRPCConfig{
			Port: viper.GetInt("GRPC_PORT"),
		},
		HTTP: HTTPConfig{
			Port: viper.GetInt("HTTP_PORT"),
		},
		Postgres: PostgresConfig{
			Host:     viper.GetString("POSTGRES_HOST"),
			Port:     viper.GetInt("POSTGRES_PORT"),
			User:     viper.GetString("POSTGRES_USER"),
			Password: viper.GetString("POSTGRES_PASSWORD"),
			DBName:   viper.GetString("POSTGRES_DB"),
			SSLMode:  viper.GetString("POSTGRES_SSLMODE"),
		},
		RabbitMQ: RabbitMQConfig{
			URL:      viper.GetString("RABBITMQ_URL"),
			Exchange: viper.GetString("RABBITMQ_EXCHANGE"),
		},
		Redis: RedisConfig{
			Addr:            viper.GetString("REDIS_ADDR"),
			Password:        viper.GetString("REDIS_PASSWORD"),
			DB:              viper.GetInt("REDIS_DB"),
			CacheTTLSeconds: viper.GetInt("REDIS_CACHE_TTL_SECONDS"),
		},
		Service: ServiceConfig{
			Name: viper.GetString("SERVICE_NAME"),
		},
		Clients: ClientsConfig{
			UserProfileAddr: viper.GetString("USER_PROFILE_SERVICE_ADDR"),
			MatchingAddr:    viper.GetString("MATCHING_SERVICE_ADDR"),
		},
		Worker: WorkerConfig{
			PoolSize:              viper.GetInt("WORKER_POOL_SIZE"),
			QueueSize:             viper.GetInt("WORKER_QUEUE_SIZE"),
			RecalcIntervalMinutes: viper.GetInt("WORKER_RECALC_INTERVAL_MINUTES"),
		},
	}

	// Apply defaults
	if cfg.GRPC.Port == 0 {
		cfg.GRPC.Port = 50053
	}
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = 8082
	}
	if cfg.Postgres.Port == 0 {
		cfg.Postgres.Port = 5432
	}
	if cfg.Postgres.SSLMode == "" {
		cfg.Postgres.SSLMode = "disable"
	}
	if cfg.RabbitMQ.URL == "" {
		cfg.RabbitMQ.URL = "amqp://guest:guest@localhost:5672/"
	}
	if cfg.RabbitMQ.Exchange == "" {
		cfg.RabbitMQ.Exchange = "dating.events"
	}
	if cfg.Redis.Addr == "" {
		cfg.Redis.Addr = "localhost:6379"
	}
	if cfg.Redis.CacheTTLSeconds == 0 {
		cfg.Redis.CacheTTLSeconds = 3600 // 1 hour
	}
	if cfg.Service.Name == "" {
		cfg.Service.Name = "recommendation-service"
	}
	if cfg.Clients.UserProfileAddr == "" {
		cfg.Clients.UserProfileAddr = "user-profile-service:50051"
	}
	if cfg.Clients.MatchingAddr == "" {
		cfg.Clients.MatchingAddr = "matching-service:50052"
	}
	if cfg.Worker.PoolSize == 0 {
		cfg.Worker.PoolSize = 8
	}
	if cfg.Worker.QueueSize == 0 {
		cfg.Worker.QueueSize = 512
	}
	if cfg.Worker.RecalcIntervalMinutes == 0 {
		cfg.Worker.RecalcIntervalMinutes = 30
	}

	return cfg, nil
}

func (c *PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}
