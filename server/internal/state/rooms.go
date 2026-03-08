package state

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/shared"
	"github.com/gregriff/vogo/shared/requests"
)

// RoomUser represents a user that is actively participating in a Room.
type RoomUser struct {
	Id       uuid.UUID
	Name     string
	joinedAt time.Time

	// PendingConnections maintains signaling state between other users. It is only used when the
	// user first joins the channel and is connecting to the other users. It maps the recipient's uuid
	// to their connection struct.
	PendingConnections *connMap
	Offers             chan requests.ConnectionWithId
}

// NewRoomUser creates a user struct for sending and receiving data to and from the room and its users.
func NewRoomUser(u *dal.User) *RoomUser {
	const maxConns = shared.ChannelCapacity - 1
	return &RoomUser{
		Id:                 u.Id,
		Name:               u.Name,
		PendingConnections: &connMap{conns: make(map[uuid.UUID]*connection, maxConns)},
		Offers:             make(chan requests.ConnectionWithId, maxConns),
	}
}

// room is a representation of a database Channel that is actively being used by one
// or more of its members for voice chat.
type room struct {
	dal.Channel

	mu    sync.Mutex
	users map[uuid.UUID]*RoomUser

	logger *slog.Logger
}

// newRoom instantiates a new Room with the user that has just joined it. This
// should only be run inside of the lock of roomMap. A parent logger
// is used to create a child logger to report events in the room.
func newRoom(c *dal.Channel, user *RoomUser, logger *slog.Logger) *room {
	users := make(map[uuid.UUID]*RoomUser, shared.ChannelCapacity)
	user.joinedAt = time.Now()
	users[user.Id] = user
	return &room{
		Channel: *c,
		users:   users,
		logger:  logger.WithGroup("room").With("owner", c.Owner, "name", c.Name),
	}
}

func (r *room) GetUser(id uuid.UUID) *RoomUser {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.users[id]
}

// Users returns a map of uuids to pointers to all users currently in the room other than omitID.
// Note that if the caller wants to use this retval at a much later time, it will not be up-to-date with
// any new room users.
func (r *room) Users(omitId uuid.UUID) map[uuid.UUID]*RoomUser {
	users := make(map[uuid.UUID]*RoomUser, shared.ChannelCapacity)
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, user := range r.users {
		if user.Id == omitId {
			continue
		}
		users[id] = user
	}
	return users
}

// addUser adds a member to the room. This should only be run inside
// of the room's lock.
func (r *room) addUser(user *RoomUser) error {
	if userCount := len(r.users); userCount >= r.Capacity {
		if userCount > r.Capacity {
			r.logger.Error("room is above capacity: %d", "roomName", r.Name, "userCount", userCount)
		}
		return fmt.Errorf("room is at capacity")
	}
	user.joinedAt = time.Now()
	r.users[user.Id] = user
	return nil
}

func (r *room) Leave(user *RoomUser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.users, user.Id)
}

func CreateOrJoinRoom(c *dal.Channel, user *RoomUser, logger *slog.Logger) (*room, error) {
	rooms := getRooms()
	rooms.mu.Lock()
	r, exists := rooms.active[c.Id]
	if !exists {
		r = newRoom(c, user, logger)
		rooms.active[c.Id] = r
		rooms.mu.Unlock()
		r.logger.Debug("created")
		return r, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rooms.mu.Unlock()
	if err := r.addUser(user); err != nil {
		return nil, fmt.Errorf("error adding participant: %w", err)
	}
	r.logger.Debug("join_event", "user_joined", user.Name)
	return r, nil
}

// roomMap stores signaling information for active Rooms. An active Room is
// a channel from the database that has been joined by one or more of its members.
// Since the database only stores info about Room members and not their connection status,
// this in-memory roomMap is the source of truth for Room connection information and
// facilitates creating Rooms and signaling between participants when they
// join or leave.
type roomMap struct {
	mu     sync.Mutex
	active map[uuid.UUID]*room
}

// Get returns a copy of a Room for a given id, returning an error if not found.
func (m *roomMap) Get(id uuid.UUID) (*room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, exists := m.active[id]; exists {
		return r, nil
	}
	return &room{}, errors.New("channel not found")
}

// Delete removes a room entry from the activeRooms map.
func (m *roomMap) Delete(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.active, id)
}

var (
	activeRooms     roomMap
	createRoomStore sync.Once
)

// getRooms returns a singleton storing active Rooms. Ensure you obtain the lock on the roomMap
// if you intend to read or modify it.
func getRooms() *roomMap {
	createRoomStore.Do(func() {
		activeRooms = roomMap{active: make(map[uuid.UUID]*room, 10)}
	})
	return &activeRooms
}
