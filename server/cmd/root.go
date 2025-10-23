// Package cmd contains the CLI setup and commands exposed to the user
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gregriff/vogo/server/config"
	"github.com/spf13/cobra"
)

var configFile string

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "vogo-server",
	Short: "Facilitates WebRTC signaling and persists call/channel state for Vogo clients",
	Long:  ``,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	if runtime.GOOS == "darwin" && os.Getenv("XDG_CONFIG_HOME") == "" {
		home, _ := os.UserHomeDir()
		os.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	}

	cobra.OnInitialize(func() {
		config.InitConfig(configFile)
	})

	configHome := os.Getenv("XDG_CONFIG_HOME")
	defaultConfigFilePath := fmt.Sprintf("%s/vogo-server/vogo-server.toml", configHome)
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", fmt.Sprintf("config file (default is %s)", defaultConfigFilePath))
}
