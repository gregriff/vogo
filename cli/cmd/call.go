//go:build cgo

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/gregriff/vogo/cli/internal/netw"
	"github.com/gregriff/vogo/shared"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var callCmd = &cobra.Command{
	Use:   "call [username]",
	Short: "Call a friend",
	Long: `Arguments:
      name    The username of the friend to call (required)
	`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("recipient must be specified as an argument")
		}

		recipient := args[0]
		if len(recipient) > shared.MaxUsernameLen {
			return fmt.Errorf("recipient string too long")
		}
		viper.Set("recipient", recipient)
		return nil
	},
	Run: callFriend,
}

func init() {
	rootCmd.AddCommand(callCmd)
}

func callFriend(_ *cobra.Command, _ []string) {
	_, vogoServer, stunServer, recipient, username, password := viper.GetBool("debug"),
		viper.GetString("servers.vogo-origin"),
		viper.GetString("servers.stun-origin"),
		viper.GetString("recipient"),
		viper.GetString("user.name"),
		viper.GetString("user.password")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	credentials := netw.NewCredentials(stunServer, vogoServer, username, password)
	err := netw.CallFriend(ctx, credentials, recipient)
	if err != nil {
		if err == io.EOF {
			return
		}
		fmt.Println(err)
	}
}
