package channel

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gregriff/vogo/cli/internal/netw"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	// _ "net/http/pprof".
)

var joinCmd = &cobra.Command{
	Use:   "join [channel]",
	Short: "Join an existing channel",
	Long: `Arguments:
      channel    The name of the channel to join (required)
	`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("channel name must be specified as an argument")
		}

		channel := args[0]
		if len(channel) > 16 {
			return fmt.Errorf("channel name too long")
		}
		viper.Set("channelName", channel)
		return nil
	},
	Run: joinChannel,
}

func init() {
	ChannelCmd.AddCommand(joinCmd)
}

func joinChannel(_ *cobra.Command, _ []string) {
	_, vogoServer, stunServer, channelName, username, password := viper.GetBool("debug"),
		viper.GetString("servers.vogo-origin"),
		viper.GetString("servers.stun-origin"),
		viper.GetString("channelName"),
		viper.GetString("user.name"),
		viper.GetString("user.password")

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	credentials := netw.NewCredentials(stunServer, vogoServer, username, password)
	err := netw.JoinChannel(ctx, credentials, channelName)
	if err != nil {
		fmt.Println(err)
	}
}
