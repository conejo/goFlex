// cmd/settings.go — show current radio settings.

package cmd

import (
	"context"
	"fmt"
	"time"

	"goFlex/config"
	"goFlex/radio"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// settingsCmd queries the radio for its current settings.
var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Show the current settings of the connected radio",
	Long:  `Connect to the configured FlexRadio and display its current settings.`,
	RunE:  runSettings,
}

func init() {
	rootCmd.AddCommand(settingsCmd)
}

func runSettings(cmd *cobra.Command, args []string) error {
	cfg := &config.Config{
		RadioAddress: viper.GetString("radio-address"),
		RadioPort:    viper.GetInt("radio-port"),
		MaxLog:       viper.GetInt("max-log"),
	}

	addr := fmt.Sprintf("%s:%d", cfg.RadioAddress, cfg.RadioPort)
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Connecting to %s ...\n", addr)

	conn, err := radio.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	fmt.Fprintf(out, "Connected!\n")
	fmt.Fprintf(out, "Radio Version: %s\n", conn.Version)
	fmt.Fprintf(out, "Client Handle: 0x%X\n", conn.Handle)
	fmt.Fprintln(out, "\n--- Radio Settings ---")

	// Single timeout context for the entire command.
	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	// Start read loop in background so responses are processed.
	collector := radio.NewSliceCollector()
	go conn.ReadLoop(func(msg radio.ParsedMessage) {
		collector.HandleStatus(msg)
	})

	// Query radio settings using the "info" command.
	done := make(chan struct{})
	var settings map[string]string

	_, err = conn.Send("info", func(code int, body string) {
		if code == 0 {
			settings = radio.ParseCommaKVs(body)
		}
		close(done)
	})
	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	select {
	case <-done:
		// Response received.
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for radio response")
	}

	if settings == nil {
		return fmt.Errorf("no settings received from radio (command may have failed)")
	}

	printSettings(out, settings)

	// --- Slice details ---
	fmt.Fprintln(out, "\n--- Slice Details ---")

	// Register as a client (required for the radio to send slice status).
	if err := conn.RegisterClient(ctx, "goFlex-settings"); err != nil {
		return fmt.Errorf("failed to register client: %w", err)
	}

	// Subscribe to slice status and wait for the radio to push current state.
	if err := conn.SubscribeSlices(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to slices: %w", err)
	}

	// Allow status messages to arrive.
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}

	slices := collector.GetSlices()
	if len(slices) == 0 {
		fmt.Fprintln(out, "  (no slices found)")
	} else {
		printSlices(out, slices)
	}

	// Unsubscribe cleanly.
	if err := conn.UnsubscribeSlices(ctx); err != nil {
		return fmt.Errorf("failed to unsubscribe from slices: %w", err)
	}

	return nil
}
