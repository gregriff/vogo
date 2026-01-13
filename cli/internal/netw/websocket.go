package netw

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// credentials are for signaling and connecting
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
	credentials *credentials,
	endpoint string,
) (*websocket.Conn, error) {
	cfg, err := newWebsocketConfig(credentials, endpoint)
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
// When an empty candidate is read, the channel is closed, signalling that ICE gather on this
// websocket is finished. If the ws is closed or there is an error while reading, the ws is closed and the loop stops.
func readCandidates(ctx context.Context, ws *websocket.Conn, ch chan webrtc.ICECandidateInit) error {
	var candidate webrtc.ICECandidateInit
	for {
		err := receiveWithContext(ctx, ws, &candidate)
		if err != nil {
			// if err == io.EOF {
			// 	return err
			// }
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

// receiveWithContext reads json into v from ws in a new goroutine and cancels
// the read if ctx is cancelled. Param v should be a pointer.
func receiveWithContext(ctx context.Context, ws *websocket.Conn, v any) error {
	var (
		recv sync.WaitGroup
		done = make(chan error, 1)
	)
	defer recv.Wait()

	recv.Go(func() {
		done <- websocket.JSON.Receive(ws, v)
	})

	select {
	case <-ctx.Done():
		ws.SetReadDeadline(time.Now()) // interrupt the read
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// closeAndWait closes the websocket. wg should be the waitgroup
// for the goroutine reading the websocket. If goroutines reading the
// websocket are using recieveWithContext, they will unblock.
func closeAndWait(ws *websocket.Conn, wg *sync.WaitGroup) {
	_ = ws.Close() // errs if already closed
	if wg != nil {
		wg.Wait()
	}
	log.Println("ws closed")
}
