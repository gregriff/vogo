package cmd

import (
	"fmt"
	"log"

	"github.com/gregriff/vogo/cli/internal/netw/crud"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	// _ "net/http/pprof".
	"github.com/gregriff/vogo/shared/validation"
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register this client with a new user",
	Args:  cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		inviteCode := viper.GetString("code")
		if inviteCode == "" {
			return fmt.Errorf("must specify an invite code to register")
		}
		return nil
	},
	Run: registerUser,
}

func init() {
	rootCmd.AddCommand(registerCmd)

	flagName := "code"
	registerCmd.PersistentFlags().String(flagName, "", "invite code for a vogo server")
	_ = viper.BindPFlag(flagName, registerCmd.PersistentFlags().Lookup(flagName))

}

func registerUser(_ *cobra.Command, _ []string) {
	_, username, password, inviteCode, vogoServer := viper.GetBool("debug"),
		viper.GetString("user.name"),
		viper.GetString("user.password"),
		viper.GetString("code"),
		viper.GetString("servers.vogo-origin")

	if err := validation.CheckUsername(username); err != nil {
		msg := fmt.Errorf("invalid username %s (%w)", username, err)
		log.Fatal(msg.Error())
	}
	if err := validation.CheckPassword(password); err != nil {
		msg := fmt.Errorf("invalid password %s (%w)", password, err)
		log.Fatal(msg.Error())
	}

	vogoClient := crud.NewClient(vogoServer, "", "")
	username, err := crud.Register(vogoClient, username, password, inviteCode)
	if err != nil {
		log.Fatal(fmt.Errorf("error during registration: %w", err).Error())
	}

	// writeErr := configs.PersistCredentialsToConfig(ConfigFile, username, friendCode)
	// if writeErr != nil {
	// 	log.Fatalf(
	// 		`error writing username to config file. please write username=%s to %s`,
	// 		username, ConfigFile,
	// 	)
	// }
	log.Printf("Now registered with username: %s", username)
}
