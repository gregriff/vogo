package dal

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/server/internal/crypto"
)

func AddInviteCode(db *sql.DB, code string) error {
	ctx := context.TODO()
	id := uuid.New()
	result, err := db.ExecContext(ctx,
		"INSERT INTO invite_codes (id, code) VALUES ($1, $2) ON CONFLICT DO NOTHING", id, code,
	)
	if err != nil {
		return err
	}

	var rows int64
	if rows, err = result.RowsAffected(); err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("invite code already exists")
	}
	return nil
}

func ValidateInviteCode(db *sql.DB, code string) error {
	ctx := context.TODO()
	if len(code) < crypto.InviteCodeLength || len(code) > crypto.InviteCodeLength {
		return fmt.Errorf("invalid length")
	}
	var registeredUserId sql.NullString

	query := "SELECT registered_user_id FROM invite_codes WHERE code = $1 LIMIT 1"
	err := db.QueryRowContext(ctx, query, code).Scan(&registeredUserId)
	if err == sql.ErrNoRows {
		return fmt.Errorf("not found in database")
	}
	if err != nil {
		return err
	}

	// if the invite code has already been used by a user to register
	if registeredUserId.Valid {
		return fmt.Errorf("invite code already used")
	}
	return nil
}
