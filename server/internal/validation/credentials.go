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
func CheckRegistrationCredentials(db *sql.DB, inviteCode, username, password string) (error, int) {
	if vErr := dal.ValidateInviteCode(db, inviteCode); vErr != nil {
		log.Printf("invite code validation error: %v", vErr)
		return errors.New("invalid invite code"), http.StatusUnauthorized
	}
	if vErr := validateUsername(username); vErr != nil {
		return fmt.Errorf("invalid username %s (%w)", username, vErr), http.StatusBadRequest
	}
	if vErr := validatePassword(password); vErr != nil {
		return fmt.Errorf("invalid password (%w)", vErr), http.StatusBadRequest
	}
	return nil, http.StatusOK
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
