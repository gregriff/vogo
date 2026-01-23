package schemas

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

// TODO:
// - channel constructor, pub/sub pattern for joining participants?
// - add participant func
// - ensure removal of channel when last participant leaves

// channel is the representation of a voice chat channel that is actively being used by its members.
type channel struct {
	Channel
	participants []ChannelUser
}

// channelMap stores signaling information for active channels. An active channel is
// a channel from the database that has been joined by one or more of its members.
// Since the database only stores info about channel members and not their connection status,
// this in-memory channelMap is the source of truth for channel connection information and
// facilitates creating active channels and signalling between participants when they
// join or leave
type channelMap struct {
	mu       sync.Mutex
	channels map[uuid.UUID]channel
}

// Add inserts a new channel into the activeChannels singleton
func (m *channelMap) Add(id uuid.UUID, c channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[id] = c
}

// Get returns a copy of a channel for a given id, returning an error if not found.
func (m *channelMap) Get(id uuid.UUID) (channel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, exists := m.channels[id]; exists {
		return c, nil
	} else {
		return channel{}, errors.New("channel not found")
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
		activeChannels = channelMap{channels: make(map[uuid.UUID]channel, 10)}
	})
	return &activeChannels
}

// TODO:
// - CreateOrJoinChannel
// - createChannel
// - joinChannel
