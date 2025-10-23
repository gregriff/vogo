// package dal is the data access layer. It contains functions that perform SQL queries and logic
// that cannot be decoupled from the queries. Files correspond to SQL tables
package dal

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/server/internal/crypto"
	"github.com/gregriff/vogo/server/internal/schemas"
)

// CreateUser adds a user to the database and associates them with their invite code.
// TODO: use transaction to rollback if invite code fails
func CreateUser(db *sql.DB, username, hashedPassword, inviteCode string) (*string, error) {
	userId := uuid.New().String()
	friendCode := crypto.GenerateUsernameSuffix()

	// Basic transaction pattern
	tx, tErr := db.Begin()
	if tErr != nil {
		return nil, tErr
	}
	defer tx.Rollback()

	err := tx.QueryRow(
		"INSERT INTO users (id, username, friend_code, password) VALUES (?, ?, ?, ?) RETURNING friend_code",
		userId,
		username,
		friendCode,
		hashedPassword,
	).Scan(&friendCode)
	if err != nil {
		return nil, fmt.Errorf("error inserting user: %w", err)
	}

	// update invite code
	result, err := tx.Exec(
		"UPDATE invite_codes SET registered_user_id = ? WHERE code = ? AND registered_user_id IS NULL",
		userId, inviteCode,
	)
	if err != nil {
		return nil, fmt.Errorf("error updating invite code: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("invite code not found or already used")
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &friendCode, nil
}

func GetUserByUsername(db *sql.DB, username string) (*schemas.User, error) {
	var user schemas.User

	query := "SELECT id, username, password, created_at FROM users WHERE username = ?"
	err := db.QueryRow(query, username).Scan(&user.Id, &user.Name, &user.Password, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", username)
		}
		return nil, fmt.Errorf("error querying user: %w", err)
	}
	return &user, nil
}

func GetUserById(db *sql.DB, id string) (*schemas.User, error) {
	var user schemas.User

	query := "SELECT * FROM users WHERE id = ?"
	err := db.QueryRow(query, id).Scan(&user.Id, &user.Name, &user.Password, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", id)
		}
		return nil, fmt.Errorf("error querying user: %w", err)
	}
	return &user, nil
}
