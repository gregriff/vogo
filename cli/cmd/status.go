package cmd

// TODO: status should authenticate and issue a JWT

import (
	"fmt"
	"log"
	"strings"

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

	printFriends(status.Friends)
	printChannels(status.Channels)
}

func printFriends(friends []crud.Friend) {
	if len(friends) == 0 {
		fmt.Println("\nNo Friends")
		return
	}

	incomingRequests := make([]crud.Friend, 0, 2)
	for _, friend := range friends {
		if friend.Status == "pending" {
			incomingRequests = append(incomingRequests, friend)
			continue
		}
	}

	// if we have no incoming requests, but do have active friendships
	if len(incomingRequests) == 0 {
		fmt.Println("\nFriends: ")
		for _, friend := range friends {
			fmt.Println(friend.Name)
		}
		return
	}

	fmt.Println("\nIncoming Friend Requests:")
	for _, req := range incomingRequests {
		fmt.Println(req.Name)
	}
}

func printChannels(channels []crud.Channel) {
	if len(channels) == 0 {
		fmt.Println("\nNo Channels")
		return
	}

	fmt.Println("\nChannels: ")
	for _, channel := range channels {
		memberNames := strings.Trim(channel.MemberNames, "{}")
		fmt.Printf("%s (%d) - members: %s\n", channel.Name, channel.Capacity, memberNames)
	}
}
