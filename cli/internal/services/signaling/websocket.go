package signaling

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"golang.org/x/net/websocket"
)

// NewWsConfig creates a new websocket.Config for the vogo server for a specific endpoint, with basic auth.
func NewWsConfig(baseUrl, username, password, endpoint string) (*websocket.Config, error) {
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

// WriteWS writes to a websocket connection
// TODO: can replace with websocket.JSON.Send()
func WriteWS(ws *websocket.Conn, data any) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling before writing to websocket: %w", err)
	}

	n, err := ws.Write(bytes)
	log.Printf("wrote %d bytes to ws", n)
	if err != nil {
		return fmt.Errorf("error writing to websocket: %w", err)
	}
	return nil
}
