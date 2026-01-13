package cmd

import (
	"fmt"
	"log"

	"github.com/gregriff/vogo/cli/internal/netw/crud"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var inviteCmd = &cobra.Command{
	Use:   "invite [username] [channel name]",
	Short: "Invite a friend to an existing channel",
	Long: `Arguments:
      username    The name of the user to invite (required)
      channel 	  The name of the channel (required)
	`,
	Args: cobra.ExactArgs(2),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		friendName := args[0]
		if len(friendName) == 0 {
			return fmt.Errorf("must specify a friend to invite")
		}
		viper.Set("friendName", friendName)

		channelName := args[1]
		if len(channelName) > 20 {
			return fmt.Errorf("channel name too long")
		}
		if channelName == "" {
			return fmt.Errorf("must specify a channel name")
		}

		viper.Set("channelName", channelName)
		return nil
	},
	Run: inviteFriend,
}

func init() {
	rootCmd.AddCommand(inviteCmd)
}

func inviteFriend(_ *cobra.Command, _ []string) {
	_, username, password, channelName, friendName, vogoServer := viper.GetBool("debug"),
		viper.GetString("user.name"),
		viper.GetString("user.password"),
		viper.GetString("channelName"),
		viper.GetString("friendName"),
		viper.GetString("servers.vogo-origin")

	vogoClient := crud.NewClient(vogoServer, username, password)
	friend, err := crud.InviteFriend(vogoClient, channelName, friendName)
	if err != nil {
		log.Fatal(fmt.Errorf("error inviting friend: %w", err).Error())
	}

	log.Printf("Invited friend: %s", friend.Name)
}
