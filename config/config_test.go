// config_test.go — unit tests for configuration loading.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// withCleanEnv changes to a temp directory (so no .env is found) and
// unsets the three config env vars so tests start from a known state.
func withCleanEnv(t *testing.T) {
	t.Helper()
	os.Unsetenv("RADIO_ADDRESS")
	os.Unsetenv("RADIO_PORT")
	os.Unsetenv("MAX_LOG")
	origWd, _ := os.Getwd()
	os.Chdir(t.TempDir())
	t.Cleanup(func() { os.Chdir(origWd) })
}

func TestLoad_Defaults(t *testing.T) {
	withCleanEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RadioAddress != defaultRadioAddr {
		t.Fatalf("RadioAddress: want %q, got %q", defaultRadioAddr, cfg.RadioAddress)
	}
	if cfg.RadioPort != defaultRadioPort {
		t.Fatalf("RadioPort: want %d, got %d", defaultRadioPort, cfg.RadioPort)
	}
	if cfg.MaxLog != defaultMaxLog {
		t.Fatalf("MaxLog: want %d, got %d", defaultMaxLog, cfg.MaxLog)
	}
}

func TestLoad_CustomEnv(t *testing.T) {
	withCleanEnv(t)

	t.Setenv("RADIO_ADDRESS", "10.0.0.50")
	t.Setenv("RADIO_PORT", "5992")
	t.Setenv("MAX_LOG", "1000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RadioAddress != "10.0.0.50" {
		t.Fatalf("RadioAddress: want %q, got %q", "10.0.0.50", cfg.RadioAddress)
	}
	if cfg.RadioPort != 5992 {
		t.Fatalf("RadioPort: want %d, got %d", 5992, cfg.RadioPort)
	}
	if cfg.MaxLog != 1000 {
		t.Fatalf("MaxLog: want %d, got %d", 1000, cfg.MaxLog)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	withCleanEnv(t)

	t.Setenv("RADIO_PORT", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid RADIO_PORT")
	}
	if err.Error() != "invalid RADIO_PORT: strconv.Atoi: parsing \"not-a-number\": invalid syntax" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLoad_InvalidMaxLog(t *testing.T) {
	withCleanEnv(t)

	t.Setenv("MAX_LOG", "abc")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid MAX_LOG")
	}
	if err.Error() != "invalid MAX_LOG: strconv.Atoi: parsing \"abc\": invalid syntax" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLoad_EnvOverridesDotEnv(t *testing.T) {
	withCleanEnv(t)

	// Create a temporary .env file.
	dir := t.TempDir()
	dotenv := filepath.Join(dir, ".env")
	if err := os.WriteFile(dotenv, []byte("RADIO_ADDRESS=192.168.1.100\nRADIO_PORT=4993\nMAX_LOG=200\n"), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// Change working directory so godotenv.Load finds our temp .env.
	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	// Set an env var that should override the .env value.
	t.Setenv("RADIO_ADDRESS", "10.20.30.40")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Env var wins.
	if cfg.RadioAddress != "10.20.30.40" {
		t.Fatalf("RadioAddress: want %q, got %q", "10.20.30.40", cfg.RadioAddress)
	}
	// .env values used for vars not set in environment.
	if cfg.RadioPort != 4993 {
		t.Fatalf("RadioPort: want %d, got %d", 4993, cfg.RadioPort)
	}
	if cfg.MaxLog != 200 {
		t.Fatalf("MaxLog: want %d, got %d", 200, cfg.MaxLog)
	}
}

func TestLoad_DotEnvFile(t *testing.T) {
	withCleanEnv(t)

	// Create a temporary .env file.
	dir := t.TempDir()
	dotenv := filepath.Join(dir, ".env")
	if err := os.WriteFile(dotenv, []byte("RADIO_ADDRESS=192.168.1.200\nRADIO_PORT=4994\nMAX_LOG=750\n"), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// Change working directory so godotenv.Load finds our temp .env.
	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RadioAddress != "192.168.1.200" {
		t.Fatalf("RadioAddress: want %q, got %q", "192.168.1.200", cfg.RadioAddress)
	}
	if cfg.RadioPort != 4994 {
		t.Fatalf("RadioPort: want %d, got %d", 4994, cfg.RadioPort)
	}
	if cfg.MaxLog != 750 {
		t.Fatalf("MaxLog: want %d, got %d", 750, cfg.MaxLog)
	}
}

func TestLoad_PartialEnv(t *testing.T) {
	withCleanEnv(t)

	// Only set one variable; others should use defaults.
	t.Setenv("RADIO_PORT", "6000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RadioAddress != defaultRadioAddr {
		t.Fatalf("RadioAddress: want %q, got %q", defaultRadioAddr, cfg.RadioAddress)
	}
	if cfg.RadioPort != 6000 {
		t.Fatalf("RadioPort: want %d, got %d", 6000, cfg.RadioPort)
	}
	if cfg.MaxLog != defaultMaxLog {
		t.Fatalf("MaxLog: want %d, got %d", defaultMaxLog, cfg.MaxLog)
	}
}

func TestLoad_EmptyEnvVars(t *testing.T) {
	withCleanEnv(t)

	// Empty strings should fall back to defaults.
	t.Setenv("RADIO_ADDRESS", "")
	t.Setenv("RADIO_PORT", "")
	t.Setenv("MAX_LOG", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RadioAddress != defaultRadioAddr {
		t.Fatalf("RadioAddress: want %q, got %q", defaultRadioAddr, cfg.RadioAddress)
	}
	if cfg.RadioPort != defaultRadioPort {
		t.Fatalf("RadioPort: want %d, got %d", defaultRadioPort, cfg.RadioPort)
	}
	if cfg.MaxLog != defaultMaxLog {
		t.Fatalf("MaxLog: want %d, got %d", defaultMaxLog, cfg.MaxLog)
	}
}
