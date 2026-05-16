// config.go — application configuration from environment / .env file.

package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const (
	DefaultRadioAddr = "192.168.50.151"
	DefaultRadioPort = 4992
	DefaultMaxLog    = 500
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

	addr := getEnv("RADIO_ADDRESS", DefaultRadioAddr)
	port, err := getEnvInt("RADIO_PORT", DefaultRadioPort)
	if err != nil {
		return nil, fmt.Errorf("invalid RADIO_PORT: %w", err)
	}
	maxLog, err := getEnvInt("MAX_LOG", DefaultMaxLog)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_LOG: %w", err)
	}

	return &Config{
		RadioAddress: addr,
		RadioPort:    port,
		MaxLog:       maxLog,
	}, nil
}

// FromViper builds a Config from Viper settings (flags, env, config file).
// This is the preferred path when running through the CLI.
func FromViper(v *viper.Viper) *Config {
	return &Config{
		RadioAddress: v.GetString("radio-address"),
		RadioPort:    v.GetInt("radio-port"),
		MaxLog:       v.GetInt("max-log"),
	}
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
