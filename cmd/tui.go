// cmd/tui.go — start the interactive TUI.

package cmd

import (
	"fmt"

	"goFlex/app"
	"goFlex/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// tuiCmd starts the Bubble Tea TUI.
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Start the interactive terminal UI",
	Long:  `Launch the Bubble Tea TUI for discovering and connecting to FlexRadio devices.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.FromViper(viper.GetViper())
		if err := app.RunWithConfig(cfg); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
	rootCmd.Run = tuiCmd.Run // default command
}
