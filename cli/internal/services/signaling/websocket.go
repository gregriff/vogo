package signaling

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// NewConnection creates a websocket connection to the vogo server to a given endpoint,
// with http basic auth headers.
func NewConnection(
	ctx context.Context,
	baseUrl, username, password, endpoint string,
) (*websocket.Conn, error) {
	cfg, err := newWebsocketConfig(baseUrl, username, password, endpoint)
	if err != nil {
		return nil, err
	}
	ws, err := cfg.DialContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error dialing ws: %w", err)
	}
	return ws, nil
}

// newWebsocketConfig creates a new websocket.Config for the vogo server for a specific endpoint, with basic auth.
func newWebsocketConfig(baseUrl, username, password, endpoint string) (*websocket.Config, error) {
	loc := strings.Replace(baseUrl, "http", "ws", 1) + endpoint
	log.Println("ws url: ", loc)

	cfg, err := websocket.NewConfig(loc, "app://vogo") // no real origin b/c we're not a browser
	if err != nil {
		return nil, err
	}

	// set basic auth for the http request that initates the ws connection
	auth := username + ":" + password
	auth = base64.StdEncoding.EncodeToString([]byte(auth))
	cfg.Header.Set("Authorization", "Basic "+auth)

	return cfg, nil
}

// ReadCandidates reads from ws in a loop, sending candidates read to the channel ch.
// When an empty candidate is read, the channel is closed, signalling that ICE gather on this
// websocket is finished. If the ws is closed or there is an error while reading, the ws is closed and the loop stops.
func ReadCandidates(ws *websocket.Conn, ch chan webrtc.ICECandidateInit) {
	var candidate webrtc.ICECandidateInit
	for {
		err := websocket.JSON.Receive(ws, &candidate)
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Printf("error reading from ws: %v", err)
			if closeErr := ws.Close(); closeErr != nil {
				log.Printf("error closing ws: %v", closeErr)
			}
			return
		}

		if candidate.Candidate == "" {
			close(ch)
			log.Println("ice gather completed")
			return
		}
		ch <- candidate
	}
}

// CloseAndWait closes the websocket, unblocking any reads on it. wg should be the waitgroup
// for the goroutine reading the websocket.
func CloseAndWait(ws *websocket.Conn, wg *sync.WaitGroup) {
	if err := ws.Close(); err != nil {
		log.Printf("error closing ws in defer: %v", err)
	}
	wg.Wait()
}
