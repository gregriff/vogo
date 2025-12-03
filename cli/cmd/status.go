package cmd

// TODO: status should authenticate and issue a JWT

import (
	"fmt"
	"log"

	"github.com/gregriff/vogo/cli/internal/netw/crud"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	// _ "net/http/pprof".
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check for pending calls or open channels",
	Args:  cobra.NoArgs,
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

	vogoClient := crud.NewClient(vogoServer, username, password)
	status, err := crud.Status(vogoClient)
	if err != nil {
		log.Printf("error fetching status: %v", err)
		return
	}

	if len(status.Friends) == 0 {
		fmt.Println("\nNo Friends")
		return
	}
	fmt.Println("\nFriends: ")
	for _, friend := range status.Friends {
		fmt.Println(friend.Name)
	}

	if len(status.Channels) == 0 {
		fmt.Println("\nNo Channels")
		return
	}
	fmt.Println("\nChannels: ")
	for _, channel := range status.Channels {
		fmt.Printf("%s (%d)\n", channel.Name, channel.Capacity)
		for _, member := range channel.MemberNames {
			fmt.Printf("- %s\n", member)
		}
	}
}
