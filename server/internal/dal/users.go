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

// GetUser returns a user from the database given their username.
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

// GetUserWithPassword returns a friend from the database with their hashed password given their username.
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
// Use pending to control returning incoming friend requests.
func GetFriends(db *sql.DB, userId string, pending bool) ([]public.Friend, error) {
	friends := make([]public.Friend, 0, 10)

	template := `
    	SELECT u.username, f.status
        FROM friendships f
        JOIN users u ON u.id = CASE WHEN f.user_one = $1 THEN f.user_two ELSE f.user_one END
        WHERE (f.user_one = $1 OR f.user_two = $1) AND %s
    `
	var filter string
	if pending { // also return incoming friend requests
		filter = "(status = 'accepted' OR (status = 'pending' AND added_by != $1))"
	} else {
		filter = "status = 'accepted'"
	}

	query := fmt.Sprintf(template, filter)
	rows, err := db.Query(query, userId)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var f public.Friend
		if err := rows.Scan(&f.Name, &f.Status); err != nil {
			return nil, err
		}
		friends = append(friends, f)
	}

	return friends, rows.Err()
}

// GetChannels returns the channels a user with a given id is a member of.
// The result contains the user names of the channel members as a property of each channel.
func GetChannels(db *sql.DB, userId string) ([]public.Channel, error) {
	channels := make([]public.Channel, 0, 10)
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

// AddFriend adds a friend with a given name.
func AddFriend(db *sql.DB, userId uuid.UUID, friendName string) (*public.User, error) {
	friend := public.User{}

	dbFriend, err := GetUser(db, friendName)
	if err != nil {
		return &friend, fmt.Errorf("friend not found: %w", err)
	}

	// if the request is already pending, update it to accepted
	query := `
		INSERT INTO friendships (user_one, user_two, status, added_by)
		VALUES (LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid), 'pending', $1)
		ON CONFLICT (user_one, user_two)
		DO UPDATE SET status = 'accepted'
    	WHERE friendships.status = 'pending'
       `
	_, err = db.Exec(query, userId, dbFriend.Id)
	if err != nil {
		return nil, fmt.Errorf("error during add friend query: %w", err)
	}
	friend.Name = dbFriend.Name
	return &friend, nil
}

// AreFriends returns true if the two users are friends.
func AreFriends(db *sql.DB, userId, friendId uuid.UUID) (bool, error) {
	query := `
	    SELECT EXISTS(
	        SELECT 1 FROM friendships
	        WHERE (user_one, user_two) = (LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid))
	        AND status = 'accepted'
	        AND whos_blocked IS NULL
		)`

	var areFriends bool
	err := db.QueryRow(query, userId, friendId).Scan(&areFriends)
	if err != nil {
		return false, fmt.Errorf("query error: %w", err)
	}

	return areFriends, nil
}
