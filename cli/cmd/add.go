package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gregriff/vogo/cli/internal/netw/crud"
	"github.com/gregriff/vogo/shared"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var addFriendCmd = &cobra.Command{
	Use:   "add [username]",
	Short: "Add a friend given their username",
	Long: `Arguments:
      name    The username of the friend to add (required)
	`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(_ *cobra.Command, args []string) error {
		friendName := args[0]
		if len(friendName) > shared.MaxUsernameLen {
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

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	vogoClient := crud.NewClient(vogoServer, username, password)
	friend, err := crud.AddFriend(ctx, vogoClient, friendName)
	if err != nil {
		log.Fatal(fmt.Errorf("error adding friend: %w", err).Error())
	}

	log.Printf("Added friend: %s", friend.Name)
}
