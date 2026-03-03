package validation

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gregriff/vogo/server/internal/dal"
	v "github.com/gregriff/vogo/shared/validation"
)

// CheckRegistrationCredentials validates user credentials during registration.
func CheckRegistrationCredentials(db *sql.DB, inviteCode, username, password string) (int, error) {
	var err error
	if err = dal.ValidateInviteCode(db, inviteCode); err != nil {
		log.Printf("invite code validation error: %v", err)
		return http.StatusUnauthorized, errors.New("invalid invite code")
	}
	if err = v.CheckUsername(username); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid username %s (%w)", username, err)
	}
	if err = v.CheckPassword(password); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid password (%w)", err)
	}
	return http.StatusOK, nil
}
