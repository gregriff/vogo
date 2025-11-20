package cmd

// TODO: status should authenticate and issue a JWT

import (
	"log"

	"github.com/gregriff/vogo/cli/internal/netw/crud"
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
	_, username, password, vogoServer := viper.GetBool("debug"),
		viper.GetString("user.name"),
		viper.GetString("user.password"),
		viper.GetString("servers.vogo-origin")

	log.Println("getting status from vogo server")
	vogoClient := crud.NewClient(vogoServer, username, password)
	status, err := crud.Status(vogoClient)
	if err != nil {
		log.Printf("error fetching status: %v", err)
		return
	}
	log.Println(status)
}
