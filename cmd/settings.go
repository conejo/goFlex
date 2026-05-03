// cmd/settings.go — show current radio settings.

package cmd

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
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

// sliceCollector gathers status updates for slices.
type sliceCollector struct {
	mu     sync.Mutex
	slices map[string]map[string]string // sliceID -> properties
}

func newSliceCollector() *sliceCollector {
	return &sliceCollector{slices: make(map[string]map[string]string)}
}

func (sc *sliceCollector) handleStatus(msg radio.ParsedMessage) {
	if msg.Type != radio.MsgStatus {
		return
	}
	if !strings.HasPrefix(msg.Object, "slice ") {
		return
	}
	parts := strings.SplitN(msg.Object, " ", 2)
	if len(parts) != 2 {
		return
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sliceID := parts[1]
	if sc.slices[sliceID] == nil {
		sc.slices[sliceID] = make(map[string]string)
	}
	for k, v := range msg.KVs {
		sc.slices[sliceID][k] = v
	}
}

func (sc *sliceCollector) getSlices() map[string]map[string]string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	result := make(map[string]map[string]string, len(sc.slices))
	for id, props := range sc.slices {
		cp := make(map[string]string, len(props))
		for k, v := range props {
			cp[k] = v
		}
		result[id] = cp
	}
	return result
}

func runSettings(cmd *cobra.Command, args []string) error {
	cfg := &config.Config{
		RadioAddress: viper.GetString("radio-address"),
		RadioPort:    viper.GetInt("radio-port"),
		MaxLog:       viper.GetInt("max-log"),
	}

	addr := fmt.Sprintf("%s:%d", cfg.RadioAddress, cfg.RadioPort)
	fmt.Printf("Connecting to %s ...\n", addr)

	conn, err := radio.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	fmt.Printf("Connected!\n")
	fmt.Printf("Radio Version: %s\n", conn.Version)
	fmt.Printf("Client Handle: 0x%X\n", conn.Handle)
	fmt.Printf("\n--- Radio Settings ---\n")

	// Start read loop in background so responses are processed.
	collector := newSliceCollector()
	go conn.ReadLoop(func(msg radio.ParsedMessage) {
		collector.handleStatus(msg)
	})

	// Query radio settings using the "info" command
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	var settings map[string]string

	_, err = conn.Send("info", func(code int, body string) {
		if code == 0 {
			settings = parseCommaKVs(body)
		}
		close(done)
	})
	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	select {
	case <-done:
		// Response received
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for radio response")
	}

	if settings == nil {
		return fmt.Errorf("no settings received from radio (command may have failed)")
	}

	// Print settings in a formatted way
	printSettings(settings)

	// --- Slice details ---
	fmt.Printf("\n--- Slice Details ---\n")

	// Register as a client (required for the radio to send slice status).
	guiClientID := "goFlex-settings"
	doneClient := make(chan struct{})
	conn.Send(fmt.Sprintf("client gui %s", guiClientID), func(code int, body string) {
		close(doneClient)
	})
	select {
	case <-doneClient:
	case <-time.After(2 * time.Second):
	}

	// Subscribe to slice status and wait for the radio to push current state.
	_, err = conn.Send("sub slice all", nil)
	if err != nil {
		return fmt.Errorf("failed to subscribe to slices: %w", err)
	}
	time.Sleep(1 * time.Second) // allow status messages to arrive

	slices := collector.getSlices()
	if len(slices) == 0 {
		fmt.Printf("  (no slices found)\n")
	} else {
		printSlices(slices)
	}

	// Unsubscribe cleanly.
	conn.Send("unsub slice all", nil)

	return nil
}

// parseCommaKVs splits a comma-separated "key=val,key2=val2" string into a map.
// Values may be quoted with double quotes; quotes are stripped.
func parseCommaKVs(body string) map[string]string {
	kvs := make(map[string]string)
	for _, token := range strings.Split(body, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		eq := strings.IndexByte(token, '=')
		if eq < 0 {
			kvs[token] = ""
			continue
		}
		key := strings.TrimSpace(token[:eq])
		val := strings.TrimSpace(token[eq+1:])
		// Strip surrounding quotes if present
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		kvs[key] = val
	}
	return kvs
}

func printSettings(settings map[string]string) {
	// Define key settings to show first
	priorityKeys := []string{
		"callsign", "name", "model", "chassis_serial",
		"software_ver", "region", "ip", "mac",
		"num_slice", "num_tx", "num_scu", "gps",
	}

	shown := make(map[string]bool)

	// Print priority keys first
	for _, key := range priorityKeys {
		if val, ok := settings[key]; ok {
			label := strings.Title(strings.ReplaceAll(key, "_", " "))
			fmt.Printf("  %-24s: %s\n", label, val)
			shown[key] = true
		}
	}

	// Print remaining keys
	fmt.Printf("\n--- Additional Settings ---\n")
	for key, val := range settings {
		if !shown[key] {
			fmt.Printf("  %-24s: %s\n", key, val)
		}
	}
}

// printSlices displays each slice's frequency, mode, and other key properties.
func printSlices(slices map[string]map[string]string) {
	// Sort slice IDs numerically for consistent ordering.
	ids := make([]string, 0, len(slices))
	for id := range slices {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		// Try numeric sort, fall back to string sort.
		ni, ei := strconv.Atoi(ids[i])
		nj, ej := strconv.Atoi(ids[j])
		if ei == nil && ej == nil {
			return ni < nj
		}
		return ids[i] < ids[j]
	})

	for _, id := range ids {
		props := slices[id]
		fmt.Printf("\n  Slice %s\n", id)

		// Priority fields to show first
		priority := []string{"RF_frequency", "mode", "filter_lo", "filter_hi", "active", "tx", "pan", "ant_list", "step_list"}
		shown := make(map[string]bool)
		for _, key := range priority {
			if val, ok := props[key]; ok {
				label := strings.Title(strings.ReplaceAll(key, "_", " "))
				fmt.Printf("    %-18s: %s\n", label, val)
				shown[key] = true
			}
		}

		// Remaining fields
		var rest []string
		for k := range props {
			if !shown[k] {
				rest = append(rest, k)
			}
		}
		sort.Strings(rest)
		for _, k := range rest {
			fmt.Printf("    %-18s: %s\n", k, props[k])
		}
	}
}
