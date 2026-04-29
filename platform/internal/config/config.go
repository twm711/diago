package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPAddr    string
	MySQLDSN    string
	CORSOrigins []string
}

func Load() Config {
	return Config{
		HTTPAddr:    getEnv("HTTP_ADDR", ":8080"),
		MySQLDSN:    getEnv("MYSQL_DSN", "root:root@tcp(127.0.0.1:3306)/aicc?parseTime=true&charset=utf8mb4&loc=Local"),
		CORSOrigins: splitCSV(getEnv("CORS_ORIGINS", "http://localhost:5173")),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
