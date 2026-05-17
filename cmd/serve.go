// cmd/serve.go — start the HTMX web UI.

package cmd

import (
	"fmt"

	"goFlex/config"
	"goFlex/web"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd starts the HTMX web UI server.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTMX web UI",
	Long:  `Launch the HTMX web interface for discovering and connecting to FlexRadio devices.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.FromViper(viper.GetViper())
		if err := web.Serve(cfg); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
