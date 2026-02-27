package validation

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/gregriff/vogo/server/internal/dal"
)

var validCharsUsername = regexp.MustCompile(`^[A-Za-z\d@$!%*?&]+$`)
var validCharsPassword = regexp.MustCompile(`^[A-Za-z\d@$!%*?&#]+$`)

// CheckRegistrationCredentials validates user credentials during registration
func CheckRegistrationCredentials(db *sql.DB, inviteCode, username, password string) (int, error) {
	var err error
	if err = dal.ValidateInviteCode(db, inviteCode); err != nil {
		log.Printf("invite code validation error: %v", err)
		return http.StatusUnauthorized, errors.New("invalid invite code")
	}
	if err = validateUsername(username); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid username %s (%w)", username, err)
	}
	if err = validatePassword(password); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid password (%w)", err)
	}
	return http.StatusOK, nil
}

// validateUsername returns user-friendly errors
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

// validatePassword returns user-friendly errors
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
