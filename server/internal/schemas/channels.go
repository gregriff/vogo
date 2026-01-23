package schemas

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

// TODO:
// - pub/sub pattern for joining participants?
// - ensure removal of channel when last participant leaves

// channel is the representation of a voice chat channel that is actively being used by its members.
type channel struct {
	Channel

	mu           sync.Mutex
	participants []ChannelUser
}

// newActiveChannel instantiates an active channel with the user that has just joined it. This
// should only be run inside of the lock of activeChannels
func newActiveChannel(c *Channel, u *ChannelUser) *channel {
	ps := make([]ChannelUser, 0, 6)
	u.joinedAt = time.Now()
	ps = append(ps, *u)
	return &channel{
		Channel:      *c,
		participants: ps,
	}
}

// addParticipant adds a member to the active channel. This should only be run inside
// of the lock of activeChannels
func (c *channel) addParticipant(u *User) error {
	if userCount := len(c.participants); userCount >= c.Capacity {
		if userCount > c.Capacity {
			fmt.Printf("ERROR!! channel %v is above capacity: %d", c, userCount)
		}
		return fmt.Errorf("channel is at capacity")
	}
	newUser := ChannelUser{User: u.User, Id: u.Id, joinedAt: time.Now()}
	c.participants = append(c.participants, newUser)
	return nil

}

// channelMap stores signaling information for active channels. An active channel is
// a channel from the database that has been joined by one or more of its members.
// Since the database only stores info about channel members and not their connection status,
// this in-memory channelMap is the source of truth for channel connection information and
// facilitates creating active channels and signalling between participants when they
// join or leave
type channelMap struct {
	mu       sync.Mutex
	channels map[uuid.UUID]*channel
}

// Add inserts a new channel into the activeChannels singleton
func (m *channelMap) Add(c *channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[c.Id] = c
}

// Get returns a copy of a channel for a given id, returning an error if not found.
func (m *channelMap) Get(id uuid.UUID) (*channel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, exists := m.channels[id]; exists {
		return c, nil
	} else {
		return &channel{}, errors.New("channel not found")
	}
}

// Delete removes a channel entry from the activeChannels map
func (m *channelMap) Delete(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.channels, id)
}

var (
	activeChannels     channelMap
	createChannelStore sync.Once
)

// GetActiveChannels returns a singleton storing active channels.
func GetActiveChannels() *channelMap {
	createChannelStore.Do(func() {
		activeChannels = channelMap{channels: make(map[uuid.UUID]*channel, 10)}
	})
	return &activeChannels
}

// CreateOrJoinChannel creates a channel goroutine that uses sync.Cond/Wait
// to wait until another JoinChannel() from a new participant wakes it.
// that goroutine will then broadcast to all idle JoinChannel
// goroutines of the current participants, which will then communicate their Sd's to the
// joining participant. when a participant leaves or disconnects, a similar thing will happen.
// All of this uses the channel struct's one mutex.
func CreateOrJoinChannel(c *Channel, user *User, sd *webrtc.SessionDescription) uuid.UUID {
	ac := GetActiveChannels()
	ac.mu.Lock()
	channel, exists := ac.channels[c.Id]
	if !exists {
		cu := ChannelUser{User: user.User, Sd: sd, Id: user.Id}
		channel = createChannel(ac, c, &cu)
		ac.mu.Unlock()
		return channel.Id
	}
	ac.mu.Unlock()
	// TODO: impl join channel, using the channels lock, impl sync broadcasting within channel struct
	return channel.Id

}

// createChannel, called only by CreateOrJoinChannel inside of a lock, creates an active channel
// when the first member joins it, saving that users info to the struct.
func createChannel(cm *channelMap, c *Channel, cu *ChannelUser) *channel {
	channel := newActiveChannel(c, cu)
	cm.Add(channel)
	return channel
}
