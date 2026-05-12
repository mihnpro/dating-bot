package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	HTTP     HTTPConfig
	Postgres PostgresConfig
	RabbitMQ RabbitMQConfig
	Service  ServiceConfig
	Clients  ClientsConfig
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

type ServiceConfig struct {
	Name string
}

type ClientsConfig struct {
	UserProfileAddr string // gRPC address for user-profile-service
	ChatServiceURL  string // HTTP base URL for chat-service
	GatewayURL      string // HTTP base URL for gateway-service internal API
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
		Service: ServiceConfig{
			Name: viper.GetString("SERVICE_NAME"),
		},
		Clients: ClientsConfig{
			UserProfileAddr: viper.GetString("USER_PROFILE_SERVICE_ADDR"),
			ChatServiceURL:  viper.GetString("CHAT_SERVICE_URL"),
			GatewayURL:      viper.GetString("GATEWAY_SERVICE_URL"),
		},
	}

	// Defaults
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = 8087
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
	if cfg.Service.Name == "" {
		cfg.Service.Name = "notification-service"
	}
	if cfg.Clients.UserProfileAddr == "" {
		cfg.Clients.UserProfileAddr = "user-profile-service:50051"
	}
	if cfg.Clients.ChatServiceURL == "" {
		cfg.Clients.ChatServiceURL = "http://chat-service:8083"
	}
	if cfg.Clients.GatewayURL == "" {
		cfg.Clients.GatewayURL = "http://gateway-service:8086"
	}

	return cfg, nil
}

func (c *PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}
