package cmd

// TODO: status should authenticate and issue a JWT

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	// _ "net/http/pprof".
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check for pending calls or open channels",
	Args:  cobra.MaximumNArgs(0),
	Run:   getStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func getStatus(_ *cobra.Command, _ []string) {
	_, vogoServer := viper.GetBool("debug"), viper.GetString("vogo-server")

	log.Println("getting status from vogo server")
	res, hErr := http.Get(fmt.Sprintf("http://%s/status", vogoServer))
	if hErr != nil {
		panic(hErr)
	}
	log.Println("status res: ", res)
	// TODO:
}
