package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	GRPC            GRPCConfig
	HTTP            HTTPConfig
	Postgres        PostgresConfig
	RabbitMQ        RabbitMQConfig
	Service         ServiceConfig
	UserProfileAddr string
}

type GRPCConfig struct{ Port int }
type HTTPConfig struct{ Port int }

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RabbitMQConfig struct{ URL string }
type ServiceConfig struct{ Name string }

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
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		} else {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	cfg := &Config{
		GRPC: GRPCConfig{Port: viper.GetInt("GRPC_PORT")},
		HTTP: HTTPConfig{Port: viper.GetInt("HTTP_PORT")},
		Postgres: PostgresConfig{
			Host:     viper.GetString("POSTGRES_HOST"),
			Port:     viper.GetInt("POSTGRES_PORT"),
			User:     viper.GetString("POSTGRES_USER"),
			Password: viper.GetString("POSTGRES_PASSWORD"),
			DBName:   viper.GetString("POSTGRES_DB"),
			SSLMode:  viper.GetString("POSTGRES_SSLMODE"),
		},
		RabbitMQ:        RabbitMQConfig{URL: viper.GetString("RABBITMQ_URL")},
		Service:         ServiceConfig{Name: viper.GetString("SERVICE_NAME")},
		UserProfileAddr: viper.GetString("USER_PROFILE_SERVICE_ADDR"),
	}

	if cfg.GRPC.Port == 0 {
		cfg.GRPC.Port = 50052
	}
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = 8081
	}
	if cfg.Postgres.Port == 0 {
		cfg.Postgres.Port = 5432
	}
	if cfg.Postgres.SSLMode == "" {
		cfg.Postgres.SSLMode = "disable"
	}
	if cfg.RabbitMQ.URL == "" {
		cfg.RabbitMQ.URL = "amqp://localhost:5672"
	}
	if cfg.Service.Name == "" {
		cfg.Service.Name = "matching-service"
	}
	if cfg.UserProfileAddr == "" {
		cfg.UserProfileAddr = "user-profile-service:50051"
	}

	return cfg, nil
}

func (c *PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}
