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
	Use:   "add-friend",
	Short: "Add a friend given their username",
	Args:  cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		friendName := viper.GetString("name")
		if friendName == "" {
			return fmt.Errorf("must specify a friend's username")
		}
		return nil
	},
	Run: addFriend,
}

func init() {
	rootCmd.AddCommand(addFriendCmd)
	var flagName string

	flagName = "name"
	addFriendCmd.PersistentFlags().String(flagName, "", "username of a friend")
	_ = viper.BindPFlag(flagName, addFriendCmd.PersistentFlags().Lookup(flagName))

}

func addFriend(_ *cobra.Command, _ []string) {
	_, username, password, friendName, vogoServer := viper.GetBool("debug"),
		viper.GetString("user.name"),
		viper.GetString("user.password"),
		viper.GetString("name"),
		viper.GetString("servers.vogo-origin")

	vogoClient := crud.NewClient(vogoServer, username, password)
	friend, err := crud.AddFriend(vogoClient, friendName)
	if err != nil {
		log.Fatal(fmt.Errorf("error adding friend: %w", err).Error())
	}

	log.Printf("Added friend: %s", friend.Name)
}
