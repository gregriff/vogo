package validation

import (
	"fmt"
	"regexp"
)

const MaxUsernameLen = 16
const MaxPasswordLen = 30

var validCharsUsername = regexp.MustCompile(`^[A-Za-z\d@$!%*?&]+$`)
var validCharsPassword = regexp.MustCompile(`^[A-Za-z\d@$!%*?&#]+$`)

func CheckUsername(username string) error {
	if len(username) == 0 {
		return fmt.Errorf("empty username")
	}
	if len(username) > MaxUsernameLen {
		return fmt.Errorf("username too long. Must be %d characters or less", MaxUsernameLen)
	}
	if valid := validCharsUsername.MatchString(username); !valid {
		return fmt.Errorf("invalid character(s) detected. only normal characters, numbers, and some symbols (no #) allowed")
	}
	return nil
}

func CheckPassword(password string) error {
	if len(password) == 0 {
		return fmt.Errorf("empty password. please ensure it's your config file")
	}
	if len(password) > MaxPasswordLen {
		return fmt.Errorf("password too long. Must be %d characters or less", MaxPasswordLen)
	}
	if valid := validCharsPassword.MatchString(password); !valid {
		return fmt.Errorf("invalid character(s) detected. only normal characters, numbers, and some symbols allowed")
	}
	return nil
}
