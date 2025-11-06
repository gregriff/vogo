// Package cmd contains the CLI setup and commands exposed to the user
package cmd

import (
	"fmt"
	"log"

	"github.com/gregriff/vogo/server/configs"
	"github.com/spf13/cobra"
)

var ConfigFile string

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
		log.Fatal(err.Error())
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

		configs.ConfigurePostgres()
	})

	configDir := configs.GetConfigDir()
	defaultConfigFilePath := fmt.Sprintf("%s/vogo-server.toml", configDir)
	rootCmd.PersistentFlags().StringVar(&ConfigFile, "config", defaultConfigFilePath, "config file")

}
