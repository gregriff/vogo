package schemas

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type NewUserMessage struct {
	Username string
	Sd       webrtc.SessionDescription
}

// BulkConnectionMessage is sent to the client when they need to start connecting with multiple
// users in a room. This happens when a client joins a room and there are already users in it
type BulkConnectionMessage struct {
	Users []NewUserMessage
}

type IceGatherMessage struct {
	Username  string
	Candidate webrtc.ICECandidateInit
}

type roomEventType string

const (
	JoinEvent         roomEventType = "JOIN"
	ExitEvent         roomEventType = "EXIT"
	ICECandidateEvent roomEventType = "ICE"
)
const roomEventChannelSize = 10

type roomEvent struct {
	Type roomEventType
	User *RoomUser

	Candidate webrtc.ICECandidateInit
	CreatedAt time.Time
}

// RoomUser represents a user that is actively participating in a Room.
type RoomUser struct {
	Id       uuid.UUID
	Name     string
	Sd       *webrtc.SessionDescription
	joinedAt time.Time

	// Events recieves room events such as when a participant joins or leaves. It is used
	// to perform webrtc functionality.
	Events chan roomEvent
}

// NewRoomUser creates a user struct for sending and recieving data to and from the room and its users.
func NewRoomUser(u *User, sd *webrtc.SessionDescription) *RoomUser {
	return &RoomUser{
		Id:     u.Id,
		Name:   u.Name,
		Sd:     sd,
		Events: make(chan roomEvent, roomEventChannelSize),
	}
}

// room is a representation of a database Channel that is actively being used by one
// or more of its members for voice chat.
type room struct {
	Channel

	mu    sync.Mutex
	users map[uuid.UUID]*RoomUser
}

// newRoom instantiates a new Room with the user that has just joined it. This
// should only be run inside of the lock of roomMap
func newRoom(c *Channel, user *RoomUser) *room {
	users := make(map[uuid.UUID]*RoomUser, 6)
	user.joinedAt = time.Now()
	users[user.Id] = user
	return &room{
		Channel: *c,
		users:   users,
	}
}

// GetUsers returns pointers to all users currently in the room other than omitID,
// and a timestamp of when they were obtained
func (r *room) GetUsers(omitId uuid.UUID) ([]*RoomUser, time.Time) {
	users := make([]*RoomUser, 0, 6)
	r.mu.Lock()
	defer r.mu.Unlock()

	t := time.Now()
	for _, user := range r.users {
		if user.Id == omitId {
			continue
		}
		users = append(users, user)
	}
	return users, t
}

// addUser adds a member to the room. This should only be run inside
// of the room's lock
func (r *room) addUser(user *RoomUser) error {
	if userCount := len(r.users); userCount >= r.Capacity {
		if userCount > r.Capacity {
			fmt.Printf("ERROR!! room %v is above capacity: %d", r, userCount)
		}
		return fmt.Errorf("room is at capacity")
	}
	user.joinedAt = time.Now()
	r.users[user.Id] = user
	r.broadcast(&roomEvent{Type: "JOIN", User: user, CreatedAt: time.Now()})
	return nil
}

func (r *room) Leave(user *RoomUser) {
	r.mu.Lock()
	delete(r.users, user.Id)
	r.broadcast(&roomEvent{Type: "EXIT", User: user, CreatedAt: time.Now()})
	r.mu.Unlock()
}

// broadcast sends a roomEvent to all users in the room. It must only be used inside the room's lock.
// broadcast will not send the event to the user the event is about.
func (r *room) broadcast(event *roomEvent) {
	for _, user := range r.users {
		if user.Id == event.User.Id {
			continue
		}
		select {
		case user.Events <- *event:
		default:
			log.Printf("WARN: User %s event channel full", user.Id)
		}
	}
}

func CreateOrJoinRoom(c *Channel, user *RoomUser) (*room, error) {
	rooms := getRooms()
	rooms.mu.Lock()
	r, exists := rooms.active[c.Id]
	if !exists {
		r = newRoom(c, user)
		rooms.active[c.Id] = r
		rooms.mu.Unlock()
		return r, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rooms.mu.Unlock()
	if err := r.addUser(user); err != nil {
		return &room{}, fmt.Errorf("error adding participant: %w", err)
	}
	return r, nil
}

// roomMap stores signaling information for active Rooms. An active Room is
// a channel from the database that has been joined by one or more of its members.
// Since the database only stores info about Room members and not their connection status,
// this in-memory roomMap is the source of truth for Room connection information and
// facilitates creating Rooms and signalling between participants when they
// join or leave
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

// Delete removes a room entry from the activeRooms map
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
