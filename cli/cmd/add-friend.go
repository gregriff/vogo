package cmd

import (
	"fmt"
	"log"

	"github.com/gregriff/vogo/cli/internal/netw/crud"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	// _ "net/http/pprof".
)

var addFriendCmd = &cobra.Command{
	Use:   "add-friend [username]",
	Short: "Add a friend given their username",
	Long: `Arguments:
      name    The username of the friend to add (required)
	`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		friendName := args[0]
		if len(friendName) > 16 {
			return fmt.Errorf("friend's name too long")
		}
		if friendName == "" {
			return fmt.Errorf("must specify a friend's username")
		}

		viper.Set("friendName", friendName)
		return nil
	},
	Run: addFriend,
}

func init() {
	rootCmd.AddCommand(addFriendCmd)
}

func addFriend(_ *cobra.Command, _ []string) {
	_, username, password, friendName, vogoServer := viper.GetBool("debug"),
		viper.GetString("user.name"),
		viper.GetString("user.password"),
		viper.GetString("friendName"),
		viper.GetString("servers.vogo-origin")

	vogoClient := crud.NewClient(vogoServer, username, password)
	friend, err := crud.AddFriend(vogoClient, friendName)
	if err != nil {
		log.Fatal(fmt.Errorf("error adding friend: %w", err).Error())
	}

	log.Printf("Added friend: %s", friend.Name)
}
