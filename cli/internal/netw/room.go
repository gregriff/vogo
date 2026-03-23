//go:build cgo

package netw

// room.go provides the connectionMap struct, which uses Connection's to create the
// state for a an active multi-user voice channel -- a room. It provides functions to receive websocket
// messages from the vogo server, and performs actions on the correct Connection per message.

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"maps"
	"sync"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/cli/internal/audio"
	"github.com/gregriff/vogo/shared"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/wsock"
	"github.com/gregriff/vogo/shared/wsock/messages"
	"golang.org/x/net/websocket"
)

// serverConn encapsulates a websocket connection to the vogo server.
type serverConn struct {
	ws *websocket.Conn
	stunServer,
	username string
}

// connectionMap maps recipient usernames to Connections. It is used to store the state of all
// outgoing connections to users currently in the channel.
type connectionMap struct {
	mu     sync.Mutex
	conns  map[string]*Connection
	server serverConn

	// channel allows access to the audio devices and systems of the room.
	channel *audio.Channel

	// wg is used to run all sub-goroutines for the connectionMap.
	wg sync.WaitGroup
}

// InitRoom initializes the client-side state of a room--a channel with users in it. A room is
// encapsulated by a connectionMap, which stores the state of all the 1:1 voice calls between
// the room participants.
func InitRoom(
	ws *websocket.Conn,
	channel *audio.Channel,
	creds *credentials,
) *connectionMap {
	return &connectionMap{
		conns: make(map[string]*Connection, shared.ChannelCapacity),
		server: serverConn{
			ws:         ws,
			stunServer: creds.stunServer,
			username:   creds.username,
		},
		channel: channel,
		wg:      sync.WaitGroup{},
	}
}

// AddConnection creates a new *Connection for the recipient, sets up audio playback for their RemoteTrack, and
// starts eventlisteners for this connection, and begins gathering Sender and Receiver reports.
func (cm *connectionMap) AddConnection(ctx context.Context, recipientId uuid.UUID, recipientName string) *Connection {
	c := NewConnection(recipientId, recipientName, cm.server.stunServer, cm.channel.Mic.Track())
	cm.channel.AddPeer(c.Pc)
	cm.Update(recipientName, c)
	cm.wg.Go(func() {
		err := c.HandleEvents(ctx, recipientName)
		if err == io.EOF { // pc closed
			cm.Delete(recipientName)
		}
	})

	cm.wg.Go(func() {
		_ = c.CollectSenderReports()
	})

	// TODO: this will need to be appended to a struct that stores them for UI to pull from.
	cm.wg.Go(func() {
		_ = c.CollectReceiverReports()
	})
	return c
}

// Get returns a *Connection if key is in the connectionMap. Nil if not.
func (cm *connectionMap) Get(key string) *Connection {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.conns[key]
}

// Update inserts a new Connection into the map given the recipient's name, overwriting it if already present.
func (cm *connectionMap) Update(key string, c *Connection) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.conns[key] = c
}

// Len returns the number of Connections in the connectionMap.
func (cm *connectionMap) Len() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.conns)
}

// TODO: do we want to call this every time a PC is closed? or retry closed conns?
func (cm *connectionMap) Delete(key string) {
	cm.mu.Lock()
	delete(cm.conns, key)
	cm.mu.Unlock()
	log.Printf("deleted %s from connMap", key)
}

// Snapshot returns a shallow copy of the connectionMap. It should be used when iteration
// over the Connections is required, and it prevents concurrent writes from causing errors.
func (cm *connectionMap) Snapshot() map[string]*Connection {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return maps.Clone(cm.conns)
}

// Uninit closes all the room's Connections and waits for any spawned goroutines
// to finish. It should be called when the client leaves the room.
func (cm *connectionMap) Uninit() {
	var wg sync.WaitGroup
	cm.mu.Lock()

	for _, c := range cm.conns {
		wg.Go(func() {
			c.Close()
		})
	}

	cm.mu.Unlock()
	wg.Wait()
}

// SendInitialCandidates sends ICEOffer candidates to every user in the room. It should be run
// when the client joins the channel, since the joiner is responsible for initiating the connections.
func (cm *connectionMap) SendInitialCandidates(ctx context.Context) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for _, c := range cm.conns {
		cm.wg.Go(func() {
			_ = sendCandidates(ctx, cm.server.ws, c, cm.server.username, wsock.ICEOffer)
		})
	}
}

// HandleMessage dispatches handlers for each incoming message in a new goroutine
// to avoid blocking the unbuffered message loop. It dispatches to handlers
// that perform operations on Connections within the connectionMap.
func (cm *connectionMap) HandleMessage(ctx context.Context, msg wsock.Message) {
	cm.wg.Go(func() {
		switch msg.Type {
		case "ice-offer", "ice-answer":
			cm.handleIceMessage(msg)
		case "answer":
			cm.handleAnswerMessage(msg)
		case "offer":
			cm.handleOfferMessage(ctx, msg)
		default:
			log.Printf("WARN: unknown message: %v", msg)
		}
	})
}

// handleOfferMessage happens when the client is already in the room and a new user joins,
// sending the client their offer.
func (cm *connectionMap) handleOfferMessage(ctx context.Context, msg wsock.Message) {
	var offer requests.ConnectionWithId
	if err := json.Unmarshal(msg.Data, &offer); err != nil {
		log.Panicf("error unmarshaling offer: %v", err)
	}

	var conn *Connection
	if conn = cm.Get(offer.From); conn != nil {
		conn.Close()
		log.Printf("recreating connection to %s", offer.From)
	}
	if cm.Len() >= audio.MaxStreams {
		// TODO: send err on chan for UI to pick up.
		log.Printf("could not accept offer from %s: already have max audio streams. num conns=%d", offer.From, cm.Len())
		return
	}
	conn = cm.AddConnection(ctx, offer.FromId, offer.From)
	log.Printf("received offer from %s, created conn", offer.From)

	// create and send answer. TODO: are retries automatic?
	err := conn.CreateAnswer(&offer.Sd)
	if err != nil { // should prob retry
		log.Printf("error creating answer: %v", err)
		conn.Close()
		return
	}

	answer := requests.Connection{From: offer.To, To: offer.From, Sd: *conn.Pc.LocalDescription()}
	bytes, err := json.Marshal(requests.ConnectionWithId{
		Connection: answer,
		FromId:     offer.ToId,
		ToId:       offer.FromId,
	})
	if err != nil {
		log.Panicf("error encoding answer: %v", err)
	}

	aMsg := wsock.Message{Type: "answer", Data: bytes}
	if err := websocket.JSON.Send(cm.server.ws, aMsg); err != nil {
		log.Printf("error sending answer: %v", err)
		conn.Close()
		return
	}
	log.Printf("sent answer (from %s) to %s to server", answer.From, answer.To)

	cm.wg.Go(func() {
		_ = sendCandidates(ctx, cm.server.ws, conn, cm.server.username, wsock.ICEAnswer)
	})
}

// handleAnswerMessage handles when the client receives an answer from a recipient.
func (cm *connectionMap) handleAnswerMessage(msg wsock.Message) {
	var answer requests.Connection
	if err := json.Unmarshal(msg.Data, &answer); err != nil {
		log.Panicf("error unmarshaling answer: %v", err)
	}

	var conn *Connection
	if conn = cm.Get(answer.To); conn == nil {
		log.Panicf("error: connection for user %s not found", answer.From)
	}
	if err := conn.Pc.SetRemoteDescription(answer.Sd); err != nil {
		log.Printf("error while setting remote description: %v", err)
		conn.Close()
		return
	}
	conn.CloseRemoteSetOnce.Do(func() { close(conn.RemoteSet) })
}

// handleIceMessage unmarshals an ICEAnswerCandidate from a peer and adds it to the
// client's set of candidates for that peer.
func (cm *connectionMap) handleIceMessage(msg wsock.Message) {
	var data messages.Candidate
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		log.Panicf("error unmarshaling %s candidate: %v", msg.Type, err)
	}

	var conn *Connection
	if conn = cm.Get(data.Username); conn == nil {
		log.Panicf("error: connection for user %s not found", data.Username)
	}

	// wait for remote description to be set
	<-conn.RemoteSet

	// todo: ensure remote description has been set before this runs
	if err := conn.Pc.AddICECandidate(data.Candidate); err != nil {
		log.Printf("error adding ICE candidate: %v", err)
	}
}
