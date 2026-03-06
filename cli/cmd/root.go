// Package cmd contains the CLI setup and commands exposed to the user
package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gregriff/vogo/cli/cmd/channel"
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
	rootCmd.AddCommand(channel.ChannelCmd)

	// deferring this allows user to override config path with cli option
	cobra.OnInitialize(func() {
		nativeFilepath, err := filepath.Abs(ConfigFile)
		if err != nil {
			log.Fatalf("error resolving config file: %v", err)
		}
		// TODO: debug
		log.Printf("using config file: %s", nativeFilepath)
		configs.Init("vogo", nativeFilepath)

		username, password := viper.GetString("user.name"), viper.GetString("user.password")
		if len(username) == 0 {
			log.Fatalf("username not found. ensure it is present in %s", ConfigFile)
		}
		if len(password) == 0 {
			log.Fatalf("password not found. ensure it is present in %s", ConfigFile)
		}
	})

	configDir := configs.Dir("vogo")
	defaultConfigFilePath := filepath.Join(configDir, "vogo.toml")
	rootCmd.PersistentFlags().StringVar(&ConfigFile, "config", defaultConfigFilePath, "config file")

	rootCmd.PersistentFlags().String("stun-server", "stun:stun.l.google.com:19302", "STUN server origin")
	rootCmd.PersistentFlags().String("vogo-server", "", "vogo server address")
	rootCmd.PersistentFlags().Bool("debug", false, "print debugging information")

	// expose to application via viper
	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	_ = viper.BindPFlag("servers.stun-origin", rootCmd.PersistentFlags().Lookup("stun-server"))
}
