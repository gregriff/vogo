// package dal is the data access layer. It contains functions that perform SQL queries and logic
// that cannot be decoupled from the queries. Files correspond to SQL tables
package dal

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/server/internal/schemas"
	"github.com/gregriff/vogo/server/internal/schemas/public"
)

// CreateUser adds a user to the database and associates them with their invite code.
func CreateUser(db *sql.DB, username, hashedPassword, inviteCode string) (*string, error) {
	userId := uuid.New()
	username = strings.ToLower(username)

	// Basic transaction pattern
	tx, tErr := db.Begin()
	if tErr != nil {
		return nil, tErr
	}
	defer tx.Rollback()

	var dbUsername string
	err := tx.QueryRow(
		"INSERT INTO users (id, username, password) VALUES ($1, $2, $3) RETURNING username",
		userId,
		username,
		hashedPassword,
	).Scan(&dbUsername)
	if err != nil {
		return nil, fmt.Errorf("error inserting user: %w", err)
	}

	// update invite code
	result, err := tx.Exec(
		"UPDATE invite_codes SET registered_user_id = $1 WHERE code = $2 AND registered_user_id IS NULL",
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
	return &dbUsername, nil
}

func GetUser(db *sql.DB, username string) (*schemas.User, error) {
	var user schemas.User
	username = strings.ToLower(username)

	query := "SELECT id, username, created_at FROM users WHERE username = $1"
	err := db.QueryRow(query, username).Scan(&user.Id, &user.Name, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", username)
		}
		return nil, fmt.Errorf("error querying user: %w", err)
	}
	return &user, nil
}

func GetUserWithPassword(db *sql.DB, username string) (*schemas.UserWithPassword, error) {
	var user schemas.UserWithPassword
	username = strings.ToLower(username)

	query := "SELECT id, username, password, created_at FROM users WHERE username = $1"
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

	query := "SELECT id, username, created_at FROM users WHERE id = $1"
	err := db.QueryRow(query, id).Scan(&user.Id, &user.Name, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", id)
		}
		return nil, fmt.Errorf("error querying user: %w", err)
	}
	return &user, nil
}

// GetFriends returns the names of the friends of a user with a given id.
func GetFriends(db *sql.DB, userId string) ([]public.User, error) {
	friends := make([]public.User, 10)
	query := `
        SELECT u.username
        FROM users u
        WHERE u.id IN (
            SELECT CASE WHEN user_one = $1 THEN user_two ELSE user_one END
            FROM friendships
            WHERE (user_one = $1 OR user_two = $1) AND status = 'accepted'
        )
    `
	rows, err := db.Query(query, userId)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var u public.User
		if err := rows.Scan(&u.Name); err != nil {
			return nil, err
		}
		friends = append(friends, u)
	}

	return friends, rows.Err()
}

// GetChannels returns the channels a user with a given id is a member of.
// The result contains the user names of the channel members as a property of each channel.
func GetChannels(db *sql.DB, userId string) ([]public.Channel, error) {
	channels := make([]public.Channel, 10)
	query := `
        SELECT
            c.id, owner_user.username as owner_name, c.name, c.description,
            c.capacity, ARRAY_AGG(u.username) as member_names
        FROM channels c
        JOIN users owner_user ON c.owner_id = owner_user.id
        JOIN channel_members cm_user ON c.id = cm_user.channel_id
        JOIN channel_members m ON c.id = m.channel_id
        JOIN users u ON m.user_id = u.id
        WHERE cm_user.user_id = $1
        GROUP BY c.id, owner_user.username, c.name, c.description, c.capacity
    `

	rows, err := db.Query(query, userId)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var ch public.Channel
		err := rows.Scan(
			&ch.Owner, &ch.Name, &ch.Description,
			&ch.Capacity, &ch.MemberNames,
		)
		if err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}

	return channels, rows.Err()
}
