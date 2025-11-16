package signaling

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"strings"

	"golang.org/x/net/websocket"
)

// NewWebsocketConn creates a websocket connection to the vogo server.
func NewWebsocketConn(
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

// ReadForever reads from ws in a loop, sending the data read to the channel ch.
// If the ws is closed or there is an error while reading, the ws is closed and the loop stops.
func ReadForever[T any](ws *websocket.Conn, data T, ch chan T) {
	var err error
	for {
		err = websocket.JSON.Receive(ws, &data)
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Printf("error reading from ws: %v", err)
			_ = ws.Close()
			return
		}
		ch <- data
	}
}
