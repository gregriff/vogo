package dal

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/shared/public"
)

// User is the database representation of public.User, without the password column.
type User struct {
	Id        uuid.UUID
	Name      string
	CreatedAt time.Time
}

// UserWithPassword is the full database representation of User.
type UserWithPassword struct {
	User

	// hashed password
	Password string
}

// CreateUser adds a user to the database and associates them with their invite code.
func CreateUser(db *sql.DB, username, hashedPassword, inviteCode string) (*string, error) {
	ctx := context.TODO()

	userId := uuid.New()
	username = strings.ToLower(username)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, err)

	var dbUsername string
	err = tx.QueryRowContext(ctx,
		"INSERT INTO users (id, username, password) VALUES ($1, $2, $3) RETURNING username",
		userId,
		username,
		hashedPassword,
	).Scan(&dbUsername)
	if err != nil {
		return nil, fmt.Errorf("error inserting user: %w", err)
	}

	// update invite code
	var result sql.Result
	result, err = tx.ExecContext(ctx,
		"UPDATE invite_codes SET registered_user_id = $1 WHERE code = $2 AND registered_user_id IS NULL",
		userId, inviteCode,
	)
	if err != nil {
		return nil, fmt.Errorf("error updating invite code: %w", err)
	}

	var rows int64
	if rows, err = result.RowsAffected(); err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}
	if rows == 0 {
		return nil, fmt.Errorf("invite code not found or already used")
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &dbUsername, nil
}

// GetUser returns a user from the database given their username.
func GetUser(db *sql.DB, username string) (*User, error) {
	ctx := context.TODO()
	username = strings.ToLower(username)

	query := "SELECT id, username, created_at FROM users WHERE username = $1"

	var user User
	err := db.QueryRowContext(ctx, query, username).Scan(&user.Id, &user.Name, &user.CreatedAt)
	return &user, err
}

// GetUserWithPassword returns a friend from the database with their hashed password given their username.
func GetUserWithPassword(db *sql.DB, username string) (*UserWithPassword, error) {
	ctx := context.TODO()
	username = strings.ToLower(username)

	query := "SELECT id, username, password, created_at FROM users WHERE username = $1"

	var user UserWithPassword
	err := db.QueryRowContext(ctx, query, username).Scan(&user.Id, &user.Name, &user.Password, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user %s not found", username)
		}
		return nil, fmt.Errorf("other error. probably wrong password")
	}
	return &user, nil
}

// Friends returns the names of the friends of the user.
// Use pending to control returning incoming friend requests.
func (u *User) Friends(db *sql.DB, pending bool) ([]public.Friend, error) {
	ctx := context.TODO()
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
	rows, err := db.QueryContext(ctx, query, u.Id.String())
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

// AddFriend adds a friend with a given name.
func (u *User) AddFriend(db *sql.DB, friendName string) (*public.User, error) {
	ctx := context.TODO()

	dbFriend, err := GetUser(db, friendName)
	if err != nil {
		return nil, fmt.Errorf("friend not found: %w", err)
	}

	// if the request is already pending, update it to accepted
	query := `
		INSERT INTO friendships (user_one, user_two, status, added_by)
		VALUES (LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid), 'pending', $1)
		ON CONFLICT (user_one, user_two)
		DO UPDATE SET status = 'accepted'
    	WHERE friendships.status = 'pending'
       `

	var friend public.User
	_, err = db.ExecContext(ctx, query, u.Id, dbFriend.Id)
	if err != nil {
		return nil, err
	}
	friend.Name = dbFriend.Name
	return &friend, nil
}

// HasFriend returns true if the users are friends.
func (u *User) HasFriend(db *sql.DB, friendId uuid.UUID) (bool, error) {
	ctx := context.TODO()

	query := `
	    SELECT EXISTS(
	        SELECT 1 FROM friendships
	        WHERE (user_one, user_two) = (LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid))
	        AND status = 'accepted'
	        AND whos_blocked IS NULL
		)`

	var areFriends bool
	err := db.QueryRowContext(ctx, query, u.Id, friendId).Scan(&areFriends)
	return areFriends, err
}
