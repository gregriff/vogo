package cmd

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

var callCmd = &cobra.Command{
	Use:   "call",
	Short: "Call a friend",
	Args:  cobra.MaximumNArgs(1),
	PreRunE: func(_ *cobra.Command, args []string) error {
		username, password := viper.GetString("user.name"), viper.GetString("user.password")
		if len(username) == 0 {
			return fmt.Errorf("username not found. ensure it is present in %s", ConfigFile)
		}
		if len(password) == 0 {
			return fmt.Errorf("password not found. ensure it is present in %s", ConfigFile)
		}

		if len(args) == 0 {
			return fmt.Errorf("recipient must be specified as an argument")
		}

		recipient := args[0]
		if len(recipient) > 16 {
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

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	credentials := netw.NewCredentials(stunServer, vogoServer, username, password)
	err := netw.CallFriend(ctx, credentials, recipient)
	if err != nil {
		fmt.Println(err)
	}
}
