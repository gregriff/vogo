package netw

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"github.com/gregriff/vogo/shared/wsock"
	"github.com/pion/webrtc/v4"
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
		return nil, fmt.Errorf("error dialing ws: %w", err)
	}
	return ws, nil
}

// newWebsocketConfig creates a new websocket.Config for the vogo server for a specific endpoint, with basic auth.
func newWebsocketConfig(c *credentials, endpoint string) (*websocket.Config, error) {
	loc := strings.Replace(c.baseURL, "http", "ws", 1) + endpoint
	log.Println("ws url: ", loc)

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

// readCandidates reads from ws in a loop, sending candidates read to the channel ch.
// When an empty candidate is read, the channel is closed, signaling that ICE gather on this
// websocket is finished. If the ws is closed or there is an error while reading, the ws is closed and the loop stops.
func readCandidates(ctx context.Context, ws *websocket.Conn, ch chan webrtc.ICECandidateInit) error {
	var candidate webrtc.ICECandidateInit
	for {
		err := wsock.ReceiveJSON(ctx, ws, &candidate)
		if err != nil {
			return fmt.Errorf("error reading from ws: %w", err)
		}

		if candidate.Candidate == "" {
			close(ch)
			log.Println("ice gather completed")
			return nil
		}

		ch <- candidate
	}
}

// closeAndWait closes the websocket. g should be the errgroup
// for the goroutine reading the websocket. If goroutines reading the
// websocket are using recieveWithContext, they will unblock.
func closeAndWait(ws *websocket.Conn, g *errgroup.Group) {
	if err := ws.Close(); err == nil { // errs if already closed
		log.Println("ws closed by client")
	}
	if g != nil {
		g.Wait()
	}
}
