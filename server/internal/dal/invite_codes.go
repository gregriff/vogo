package dal

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/server/internal/crypto"
)

func AddInviteCode(db *sql.DB, code string) error {
	id := uuid.New().String()
	result, err := db.Exec("INSERT OR IGNORE INTO invite_codes (id, code) VALUES (?, ?)", id, code)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("driver does not support RowsAffected")
	}
	if rows == 0 {
		return fmt.Errorf("invite code already exists")
	}
	return nil
}

func ValidateInviteCode(db *sql.DB, code string) error {
	if len(code) < crypto.InviteCodeLength || len(code) > crypto.InviteCodeLength {
		return errors.New("invalid length")
	}
	var registeredUserId sql.NullString

	query := "SELECT registered_user_id FROM invite_codes WHERE code = ? LIMIT 1"
	err := db.QueryRow(query, code).Scan(&registeredUserId)
	if err == sql.ErrNoRows {
		return errors.New("not found in database")
	}
	if err != nil {
		return err
	}

	// if the invite code has already been used by a user to register
	if registeredUserId.Valid {
		return errors.New("invite code already used")
	}
	return nil
}
