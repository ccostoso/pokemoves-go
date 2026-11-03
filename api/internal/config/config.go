package config

import (
	"os"
)

// Config holds the configuration values for the application.
type Config struct {
	Port string
}

// Load reads the configuration from environment variables and returns a Config instance.
func Load() Config {
	return Config{
		Port: getEnv("PORT", "8080"),
	}
}

// getEnv retrieves the value of the environment variable named by the key.
// If the variable is empty or not set, it returns the provided fallback value.
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
