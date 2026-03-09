package netw

import (
	"context"
	"encoding/base64"
	"log"
	"strings"

	"golang.org/x/net/websocket"
	"golang.org/x/sync/errgroup"
)

// credentials are for signaling and connecting.
type credentials struct {
	stunServer,
	baseURL,
	username,
	password string
}

// NewCredentials creates credentials needed to make websocket requests
// to the vogo server for signaling/connecting.
func NewCredentials(stunServer, baseURL, username, password string) *credentials {
	return &credentials{
		stunServer: stunServer,
		baseURL:    baseURL,
		username:   username,
		password:   password,
	}
}

// newWebsocket creates a websocket connection to the vogo server to a given endpoint,
// with http basic auth headers.
func newWebsocket(
	ctx context.Context,
	creds *credentials,
	endpoint string,
) (*websocket.Conn, error) {
	cfg, err := newWebsocketConfig(creds, endpoint)
	if err != nil {
		return nil, err
	}
	ws, err := cfg.DialContext(ctx)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

// newWebsocketConfig creates a new websocket.Config for the vogo server for a specific endpoint, with basic auth.
func newWebsocketConfig(c *credentials, endpoint string) (*websocket.Config, error) {
	loc := strings.Replace(c.baseURL, "http", "ws", 1) + endpoint
	log.Printf("ws url: %s", loc)

	cfg, err := websocket.NewConfig(loc, "app://vogo") // no real origin b/c we're not a browser
	if err != nil {
		return nil, err
	}

	// set basic auth for the http request that initates the ws connection
	auth := c.username + ":" + c.password
	auth = base64.StdEncoding.EncodeToString([]byte(auth))
	cfg.Header.Set("Authorization", "Basic "+auth)

	return cfg, nil
}

// closeAndWait closes the websocket. g should be the errgroup
// for the goroutine reading the websocket. If goroutines reading the
// websocket are using recieveWithContext, they will unblock.
func closeAndWait(ws *websocket.Conn, g *errgroup.Group) error {
	if err := ws.Close(); err == nil { // errs if already closed
		log.Println("ws closed by client")
	}
	if g != nil {
		return g.Wait()
	}
	return nil
}
