// cmd/root.go — root command and shared configuration.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "goFlex",
	Short: "A terminal client for FlexRadio SmartSDR devices",
	Long: `goFlex is a terminal-based client for FlexRadio SmartSDR transceivers.
It provides an interactive TUI for discovering, connecting to, and
interacting with FlexRadio devices over the network.`,
}

// Execute adds all child commands to the root command and sets flags.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.goFlex.yaml)")
	rootCmd.PersistentFlags().String("radio-address", "192.168.50.151", "IP or hostname of the FlexRadio")
	rootCmd.PersistentFlags().Int("radio-port", 4992, "TCP port for the SmartSDR protocol")
	rootCmd.PersistentFlags().Int("max-log", 500, "maximum log entries to retain in the TUI")

	viper.BindPFlag("radio-address", rootCmd.PersistentFlags().Lookup("radio-address"))
	viper.BindPFlag("radio-port", rootCmd.PersistentFlags().Lookup("radio-port"))
	viper.BindPFlag("max-log", rootCmd.PersistentFlags().Lookup("max-log"))

	viper.SetEnvPrefix("RADIO")
	viper.AutomaticEnv()
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		viper.AddConfigPath(home)
		viper.SetConfigName(".goFlex")
		viper.SetConfigType("yaml")
	}

	viper.ReadInConfig()
}
