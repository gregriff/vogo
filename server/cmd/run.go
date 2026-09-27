package cmd

import (
	server "github.com/gregriff/vogo/server/internal"
	"github.com/gregriff/vogo/server/internal/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command.
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the Vogo server",
	Args:  cobra.MaximumNArgs(0),
	PreRunE: func(_ *cobra.Command, _ []string) error {
		// TODO: prerun validation here
		return nil
	},
	Run: runServer,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runServer(_ *cobra.Command, _ []string) {
	host, port, logFile, logLevel := viper.GetString("server.host"),
		viper.GetInt("server.port"),
		viper.GetString("logging.file"),
		viper.GetString("logging.level")

	logOpts := logging.NewOpts(logFile, logLevel)
	server.CreateAndListen(host, port, logOpts)
}
