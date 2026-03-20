//go:build cgo

package channel

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gregriff/vogo/cli/internal/netw"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	// _ "net/http/pprof".
)

var joinCmd = &cobra.Command{
	Use:   "join [channel-descriptor]",
	Short: "Join an existing channel",
	Long: `Arguments:
      channel-descriptor    The owner's name and the name of the channel joined by a slash (required) [ex. 'tom/gaming']
	`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("channel descriptor must be specified as an argument")
		}

		strs := strings.Split(args[0], "/")
		if len(strs) != 2 {
			return fmt.Errorf("channel descriptor must be of the format '[owner's name]/[channel name]' (ex, 'tom/gaming')")
		}
		owner, channel := strs[0], strs[1]
		if len(owner) > 16 {
			return fmt.Errorf("owner name too long")
		}
		if len(channel) > 16 {
			return fmt.Errorf("channel name too long")
		}
		viper.Set("ownerName", owner)
		viper.Set("channelName", channel)
		return nil
	},
	Run: joinChannel,
}

func init() {
	ChannelCmd.AddCommand(joinCmd)
}

func joinChannel(_ *cobra.Command, _ []string) {
	_, vogoServer, stunServer, channelName, ownerName, username, password := viper.GetBool("debug"),
		viper.GetString("servers.vogo-origin"),
		viper.GetString("servers.stun-origin"),
		viper.GetString("channelName"),
		viper.GetString("ownerName"),
		viper.GetString("user.name"),
		viper.GetString("user.password")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	credentials := netw.NewCredentials(stunServer, vogoServer, username, password)
	err := netw.JoinChannel(ctx, credentials, ownerName, channelName)
	if err != nil {
		fmt.Println(err)
	}
}
