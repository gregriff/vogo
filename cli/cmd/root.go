// Package cmd contains the CLI setup and commands exposed to the user
package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/gregriff/vogo/cli/configs"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var ConfigFile string

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "vogo",
	Short: "Client for cross-platform P2P voice chat via WebRTC",
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

	// deferring this allows user to override config path with cli option
	cobra.OnInitialize(func() {
		log.Printf("using config file: %s", ConfigFile)
		configs.InitConfig(ConfigFile)
	})

	configDir := configs.GetConfigDir()
	defaultConfigFilePath := fmt.Sprintf("%s/vogo.toml", configDir)
	rootCmd.PersistentFlags().StringVar(&ConfigFile, "config", defaultConfigFilePath, "config file")

	rootCmd.PersistentFlags().String("stun-server", "stun:stun.l.google.com:19302", "STUN Server Origin")
	rootCmd.PersistentFlags().String("vogo-server", "", "Vogo Server Address")
	rootCmd.PersistentFlags().Bool("debug", false, "Print debugging information")

	// expose to application via viper
	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	_ = viper.BindPFlag("servers.stun-origin", rootCmd.PersistentFlags().Lookup("stun-server"))
}
