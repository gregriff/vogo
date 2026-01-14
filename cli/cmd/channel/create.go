package channel

import (
	"fmt"
	"log"

	"github.com/gregriff/vogo/cli/internal/netw/crud"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var createChannelCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a persistent voice-chat channel",
	Long: `Arguments:
      name    The name of the channel (required)
	`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		channelName := args[0]
		if len(channelName) > 20 {
			return fmt.Errorf("channel name too long")
		}
		if channelName == "" {
			return fmt.Errorf("must specify a channel name")
		}

		viper.Set("channelName", channelName)
		return nil
	},
	Run: createChannel,
}

func createChannel(_ *cobra.Command, _ []string) {
	_, username, password, channelName, vogoServer := viper.GetBool("debug"),
		viper.GetString("user.name"),
		viper.GetString("user.password"),
		viper.GetString("channelName"),
		viper.GetString("servers.vogo-origin")

	vogoClient := crud.NewClient(vogoServer, username, password)
	friend, err := crud.CreateChannel(vogoClient, channelName, "", 0)
	if err != nil {
		log.Fatal(fmt.Errorf("error creating channel: %w", err).Error())
	}

	log.Printf("Created channel: %s", friend.Name)
}
