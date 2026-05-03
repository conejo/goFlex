package cmd

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var titleCaser = cases.Title(language.English)

// formatLabel converts a snake_case key to a human-readable label.
func formatLabel(key string) string {
	return titleCaser.String(strings.ReplaceAll(key, "_", " "))
}

// printSettings displays radio settings with priority keys first.
func printSettings(w io.Writer, settings map[string]string) {
	priorityKeys := []string{
		"callsign", "name", "model", "chassis_serial",
		"software_ver", "region", "ip", "mac",
		"num_slice", "num_tx", "num_scu", "gps",
	}

	shown := make(map[string]bool)

	for _, key := range priorityKeys {
		if val, ok := settings[key]; ok {
			fmt.Fprintf(w, "  %-24s: %s\n", formatLabel(key), val)
			shown[key] = true
		}
	}

	fmt.Fprintln(w, "\n--- Additional Settings ---")
	for key, val := range settings {
		if !shown[key] {
			fmt.Fprintf(w, "  %-24s: %s\n", key, val)
		}
	}
}

// printSlices displays each slice's frequency, mode, and other key properties.
func printSlices(w io.Writer, slices map[string]map[string]string) {
	ids := make([]string, 0, len(slices))
	for id := range slices {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		ni, ei := strconv.Atoi(ids[i])
		nj, ej := strconv.Atoi(ids[j])
		if ei == nil && ej == nil {
			return ni < nj
		}
		return ids[i] < ids[j]
	})

	for _, id := range ids {
		props := slices[id]
		fmt.Fprintf(w, "\n  Slice %s\n", id)

		priority := []string{"RF_frequency", "mode", "filter_lo", "filter_hi", "active", "tx", "pan", "ant_list", "step_list"}
		shown := make(map[string]bool)
		for _, key := range priority {
			if val, ok := props[key]; ok {
				fmt.Fprintf(w, "    %-18s: %s\n", formatLabel(key), val)
				shown[key] = true
			}
		}

		var rest []string
		for k := range props {
			if !shown[k] {
				rest = append(rest, k)
			}
		}
		sort.Strings(rest)
		for _, k := range rest {
			fmt.Fprintf(w, "    %-18s: %s\n", k, props[k])
		}
	}
}
