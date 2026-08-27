// package dal is the data access layer. It contains functions that perform SQL queries and logic
// that cannot be decoupled from the queries, as well as structs representing database records.
// Filenames correspond to SQL tables.
package dal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/shared/public"
	"github.com/gregriff/vogo/shared/requests"
)

var ErrChannelNotFound = errors.New("channel not found or inviter is not owner")

// Channel is the database representation of public.Channel.
type Channel struct {
	public.Channel

	Id        uuid.UUID
	CreatedAt time.Time
}

// GetChannels returns the channels a user with a given id is a member of.
// The result contains the user names of the channel members as a property of each channel.
func GetChannels(db *sql.DB, userId string) ([]public.Channel, error) {
	ctx := context.TODO()
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

	rows, err := db.QueryContext(ctx, query, userId)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var tmpId uuid.UUID
	for rows.Next() {
		var ch public.Channel
		err = rows.Scan(
			&tmpId, &ch.Owner, &ch.Name, &ch.Description,
			&ch.Capacity, &ch.MemberNames,
		)
		if err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}

	return channels, rows.Err()
}

// CreateChannel creates a channel in the database. TODO: handle onconflict, tell user to use PUT to edit.
func CreateChannel(db *sql.DB, ownerId uuid.UUID, data requests.CreateChannel) (*public.Channel, error) {
	ctx := context.TODO()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, err)

	query := `
		INSERT INTO channels (id, owner_id, name, description, capacity)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING RETURNING id, name, description, capacity
	`

	var channel public.Channel
	var channelId uuid.UUID
	err = tx.QueryRowContext(ctx, query, uuid.New(), ownerId, data.Name, data.Description, data.Capacity).
		Scan(&channelId, &channel.Name, &channel.Description, &channel.Capacity)
	if err != nil {
		return nil, fmt.Errorf("error inserting new channel: %w", err)
	}

	query = `
		INSERT INTO channel_members (channel_id, user_id, invited_by)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`
	if _, err = tx.ExecContext(ctx, query, channelId, ownerId, ownerId); err != nil {
		return nil, fmt.Errorf("error adding creator as a member of channel: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &channel, nil
}

// InviteFriend adds a friend to an existing channel. Only the owner can invite.
func (u *User) InviteFriend(db *sql.DB, channelName, friendName string) (*public.User, error) {
	ctx := context.TODO()

	dbFriend, err := GetUser(db, friendName)
	if err != nil {
		return nil, fmt.Errorf("error querying friend: %w", err)
	}

	query := `
   		WITH channel_check AS (
          SELECT c.id
          FROM channels c
          WHERE c.owner_id = $2 AND c.name = $1
        )
        INSERT INTO channel_members (channel_id, user_id, invited_by)
        SELECT id, $3, $2
        FROM channel_check
        RETURNING channel_id;
    `

	var friend public.User
	var channelId uuid.UUID
	err = db.QueryRowContext(ctx, query, channelName, u.Id, dbFriend.Id).Scan(&channelId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrChannelNotFound
		}
		// TODO: check if user is already in channel.
		return nil, fmt.Errorf("error inserting into channel_members (user probably already in channel): w", err)
	}
	friend.Name = dbFriend.Name
	return &friend, nil
}

// GetChannelOfMember returns a channel with a given name, that is owned by ownerId and memberId
// is a member of. It prevents a member from accessing a channel of another owner with the same name.
// This is because there is a unique constraint on db::channels(owner_id, name).
func GetChannelOfMember(db *sql.DB, name string, memberId, ownerId uuid.UUID) (*Channel, error) {
	ctx := context.TODO()

	query := `
        SELECT
            c.id, c.name, c.description, c.capacity, c.created_at
        FROM channels c
        JOIN channel_members m ON c.id = m.channel_id
        WHERE m.user_id = $1 AND c.owner_id = $2 AND c.name = $3
    `

	var channel Channel
	err := db.QueryRowContext(ctx, query, memberId, ownerId, name).
		Scan(&channel.Id, &channel.Name, &channel.Description, &channel.Capacity, &channel.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("channel not found: %s", name)
		}
		return nil, err
	}
	return &channel, nil
}

func rollback(tx *sql.Tx, err error) {
	if rErr := tx.Rollback(); rErr != nil {
		log.Printf("rollback error: %v, caused by %v", rErr, err)
	}
}
