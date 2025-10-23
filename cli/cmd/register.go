package cmd

import (
	"errors"
	"fmt"
	"log"
	"regexp"

	"github.com/gregriff/vogo/cli/configs"
	"github.com/gregriff/vogo/cli/internal/services/vogo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	// _ "net/http/pprof".
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register this client with a new user",
	Args:  cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		inviteCode := viper.GetString("code")
		if inviteCode == "" {
			return fmt.Errorf("must specify an invite code to register")
		}

		friendCode := viper.GetString("friend-code")
		if friendCode != "" {
			return fmt.Errorf(
				"existing friend code detected in config file, have you already registered a user with this client? "+
					"to register a new user, please delete the friend code from your config file (%s)", ConfigFile,
			)
		}
		return nil
	},
	Run: registerUser,
}

func init() {
	rootCmd.AddCommand(registerCmd)
	var flagName string

	flagName = "code"
	registerCmd.PersistentFlags().String(flagName, "", "invite code for a vogo server")
	_ = viper.BindPFlag(flagName, registerCmd.PersistentFlags().Lookup(flagName))

}

func registerUser(_ *cobra.Command, _ []string) {
	_, username, password, inviteCode, vogoServer := viper.GetBool("debug"),
		viper.GetString("username"),
		viper.GetString("password"),
		viper.GetString("code"),
		viper.GetString("vogo-server")

	if vErr := validateUsername(username); vErr != nil {
		msg := fmt.Errorf("invalid username %s (%w)", username, vErr)
		log.Fatalf(msg.Error())
	}
	if vErr := validatePassword(password); vErr != nil {
		msg := fmt.Errorf("invalid password %s (%w)", password, vErr)
		log.Fatalf(msg.Error())
	}

	vogoClient := vogo.NewClient(vogoServer, "", "")
	username, friendCode, err := vogo.Register(*vogoClient, username, password, inviteCode)
	if err != nil {
		log.Fatalf(fmt.Errorf("error during registration: %w", err).Error())
	}

	writeErr := configs.PersistCredentialsToConfig(ConfigFile, username, friendCode)
	if writeErr != nil {
		log.Fatalf(
			`error writing username to config file. please write username=%s to %s`,
			username, ConfigFile,
		)
	}
	log.Printf("Now registered with username: %s, friend code: %s", username, friendCode)
}

var validCharsUsername = regexp.MustCompile(`^[A-Za-z\d@$!%*?&]+$`)
var validCharsPassword = regexp.MustCompile(`^[A-Za-z\d@$!%*?&#]+$`)

func validateUsername(username string) error {
	if len(username) == 0 {
		return errors.New("empty username")
	}
	if len(username) > 16 {
		return errors.New("username too long. Must be 16 characters or less")
	}
	if valid := validCharsUsername.MatchString(username); !valid {
		return errors.New("invalid character(s) detected. only normal characters, numbers, and some symbols (no #) allowed")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) == 0 {
		return errors.New("empty password. please ensure it's your config file")
	}
	if len(password) > 30 {
		return errors.New("password too long. Must be 30 characters or less")
	}
	if valid := validCharsPassword.MatchString(password); !valid {
		return errors.New("invalid character(s) detected. only normal characters, numbers, and some symbols allowed")
	}
	return nil
}
