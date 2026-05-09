package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Service  ServiceConfig
	GRPC     GRPCConfig
	HTTP     HTTPConfig
	Postgres PostgresConfig
	RabbitMQ RabbitMQConfig
	Chat     ChatConfig
	Clients  ClientsConfig
}

type GRPCConfig struct {
	Port int
}

type ClientsConfig struct {
	UserProfileServiceAddr string `mapstructure:"user_profile_service_addr"`
}

type ServiceConfig struct {
	Name string
}

type HTTPConfig struct {
	Port int
}

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

func (p *PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.DBName, p.SSLMode,
	)
}

type RabbitMQConfig struct {
	URL string
}

type ChatConfig struct {
	SecretKey   string        `mapstructure:"secret_key"`
	TokenTTLSec int           `mapstructure:"token_ttl_sec"`
	FrontendURL string        `mapstructure:"frontend_url"`
}

func Load(cfgPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	if cfgPath != "" {
		v.AddConfigPath(cfgPath)
	}
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")

	v.AutomaticEnv()

	v.SetDefault("service.name", "chat-service")
	v.SetDefault("grpc.port", 50054)
	v.SetDefault("http.port", 8083)
	v.SetDefault("clients.user_profile_service_addr", "user-profile-service:50051")
	v.SetDefault("postgres.host", "chat-postgres")
	v.SetDefault("postgres.port", 5432)
	v.SetDefault("postgres.user", "postgres")
	v.SetDefault("postgres.password", "postgres")
	v.SetDefault("postgres.dbname", "chat_db")
	v.SetDefault("postgres.sslmode", "disable")
	v.SetDefault("rabbitmq.url", "amqp://guest:guest@rabbitmq:5672/")
	v.SetDefault("chat.secret_key", "change-me-in-production")
	v.SetDefault("chat.token_ttl_sec", 3600)
	v.SetDefault("chat.frontend_url", "http://localhost:3001")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
