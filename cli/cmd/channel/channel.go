package channel

import (
	"github.com/spf13/cobra"
)

var ChannelCmd = &cobra.Command{
	Use:   "channel",
	Short: "Invoke actions on a channel",
}

func init() {
	ChannelCmd.AddCommand(createChannelCmd)
	ChannelCmd.AddCommand(inviteCmd)
}
