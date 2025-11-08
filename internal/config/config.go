package config

import (
	"os"
)

type Config struct {
	GRPCPort       string
	RedisAddr      string
	JaegerEndpoint string
	MetricsPort    string
	Environment    string
	OTLPEndpoint   string
	LogFilePath    string
}

func Load() *Config {
	return &Config{
		GRPCPort:       getEnv("GRPC_PORT", "50051"),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		JaegerEndpoint: getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces"),
		OTLPEndpoint:   getEnv("OTLP_ENDPOINT", "localhost:4317"),
		MetricsPort:    getEnv("METRICS_PORT", "2112"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		LogFilePath:    getEnv("LOG_FILE_PATH", "./logs/products-service/service.log"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
