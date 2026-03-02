package validation

import (
	"errors"
	"regexp"
)

var validCharsUsername = regexp.MustCompile(`^[A-Za-z\d@$!%*?&]+$`)
var validCharsPassword = regexp.MustCompile(`^[A-Za-z\d@$!%*?&#]+$`)

func CheckUsername(username string) error {
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

func CheckPassword(password string) error {
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
