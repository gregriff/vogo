// Package cmd contains the CLI setup and commands exposed to the user
package cmd

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/gregriff/vogo/server/configs"
	"github.com/spf13/cobra"
)

var ConfigFile string
var pprofAddr string

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "vogo-server",
	Short: "Facilitates WebRTC signaling and persists call/channel state for Vogo clients",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if pprofAddr != "" {
			log.Printf("starting pprof server at %s", pprofAddr)
			go func() {
				log.Println(http.ListenAndServe(pprofAddr, nil))
			}()
		}
	},
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// deferring this allows user to override config path with cli option
	cobra.OnInitialize(func() {
		nativeFilepath, err := filepath.Abs(ConfigFile)
		if err != nil {
			log.Fatalf("error resolving config file: %v", err)
		}
		log.Printf("using config file: %s", nativeFilepath)
		configs.Init("vogo-server", nativeFilepath)

		if err := configs.ConfigurePostgres(); err != nil {
			log.Fatal(err)
		}
	})

	configDir := configs.Dir("vogo")
	defaultConfigFilePath := filepath.Join(configDir, "vogo-server.toml")
	rootCmd.PersistentFlags().StringVar(&ConfigFile, "config", defaultConfigFilePath, "config file")

	rootCmd.PersistentFlags().StringVar(&pprofAddr, "pprof", "", "enable pprof on addr (e.g. localhost:6060)")
}
