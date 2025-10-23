package cmd

import (
	server "github.com/gregriff/vogo/server/internal"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	// _ "net/http/pprof".
)

// runCmd represents the run command.
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the Vogo server",
	Args:  cobra.MaximumNArgs(0),
	PreRunE: func(_ *cobra.Command, args []string) error {
		// TODO: prerun validation here
		return nil
	},
	Run: runServer,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runServer(_ *cobra.Command, _ []string) {
	debug, host, port := viper.GetBool("debug"),
		viper.GetString("host"),
		viper.GetInt("port")

	server.CreateAndListen(debug, host, port)
}
