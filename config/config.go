// config.go — application configuration from environment / .env file.

package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

const (
	defaultRadioAddr = "192.168.50.151"
	defaultRadioPort = 4992
	defaultMaxLog    = 500
)

// Config holds all user-configurable settings.
type Config struct {
	RadioAddress string // IP or hostname of the FlexRadio
	RadioPort    int    // TCP port (usually 4992)
	MaxLog       int    // maximum log entries to retain
}

// Load reads .env (if present) and populates a Config from environment
// variables, falling back to built-in defaults.
func Load() (*Config, error) {
	// .env is optional — ignore ErrNotExist.
	_ = godotenv.Load()

	addr := getEnv("RADIO_ADDRESS", defaultRadioAddr)
	port, err := getEnvInt("RADIO_PORT", defaultRadioPort)
	if err != nil {
		return nil, fmt.Errorf("invalid RADIO_PORT: %w", err)
	}
	maxLog, err := getEnvInt("MAX_LOG", defaultMaxLog)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_LOG: %w", err)
	}

	return &Config{
		RadioAddress: addr,
		RadioPort:    port,
		MaxLog:       maxLog,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	if v := os.Getenv(key); v != "" {
		return strconv.Atoi(v)
	}
	return fallback, nil
}
