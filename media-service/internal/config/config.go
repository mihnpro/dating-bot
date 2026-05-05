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
	Minio    MinioConfig
	Service  ServiceConfig
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

type MinioConfig struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	Bucket     string
	UseSSL     bool
	PublicHost string
}

type ServiceConfig struct {
	Name string
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

	cfg := &Config{}

	cfg.GRPC.Port = viper.GetInt("GRPC_PORT")
	if cfg.GRPC.Port == 0 {
		cfg.GRPC.Port = 50054
	}

	cfg.HTTP.Port = viper.GetInt("HTTP_PORT")
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = 8083
	}

	cfg.Postgres.Host = viper.GetString("POSTGRES_HOST")
	cfg.Postgres.Port = viper.GetInt("POSTGRES_PORT")
	if cfg.Postgres.Port == 0 {
		cfg.Postgres.Port = 5432
	}
	cfg.Postgres.User = viper.GetString("POSTGRES_USER")
	cfg.Postgres.Password = viper.GetString("POSTGRES_PASSWORD")
	cfg.Postgres.DBName = viper.GetString("POSTGRES_DB")
	cfg.Postgres.SSLMode = viper.GetString("POSTGRES_SSLMODE")
	if cfg.Postgres.SSLMode == "" {
		cfg.Postgres.SSLMode = "disable"
	}

	cfg.RabbitMQ.URL = viper.GetString("RABBITMQ_URL")
	cfg.RabbitMQ.Exchange = viper.GetString("RABBITMQ_EXCHANGE")
	if cfg.RabbitMQ.Exchange == "" {
		cfg.RabbitMQ.Exchange = "dating.events"
	}

	cfg.Minio.Endpoint = viper.GetString("MINIO_ENDPOINT")
	cfg.Minio.AccessKey = viper.GetString("MINIO_ACCESS_KEY")
	cfg.Minio.SecretKey = viper.GetString("MINIO_SECRET_KEY")
	cfg.Minio.Bucket = viper.GetString("MINIO_BUCKET")
	if cfg.Minio.Bucket == "" {
		cfg.Minio.Bucket = "photos"
	}
	cfg.Minio.UseSSL = viper.GetBool("MINIO_USE_SSL")
	cfg.Minio.PublicHost = viper.GetString("MINIO_PUBLIC_HOST")

	cfg.Service.Name = viper.GetString("SERVICE_NAME")
	if cfg.Service.Name == "" {
		cfg.Service.Name = "media-service"
	}

	return cfg, nil
}

func (c *PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}
