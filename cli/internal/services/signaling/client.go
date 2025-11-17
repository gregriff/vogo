// package signaling contains code needed to perform signaling with the vogo server during the webrtc connection process.
// It is seperate from the vogo service because eventually this will be done via websockets and behavior will be sufficiently different.
package signaling

import (
	"net/http"
	"time"

	"github.com/gregriff/vogo/cli/internal/services"
)

type Credentials struct {
	baseURL,
	username,
	password string
}

// NewCredentials creates credentials needed to make http or websocket requests
// to the vogo server for signaling/connecting.
func NewCredentials(baseURL, username, password string) *Credentials {
	return &Credentials{
		baseURL:  baseURL,
		username: username,
		password: password,
	}
}

// NewClient provides an http.Client for signaling requests
func NewClient(c Credentials) *http.Client {
	signalingTransport := services.Transport{
		BaseURL:               c.baseURL,
		Username:              c.username,
		Password:              c.password,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &signalingTransport,
	}
}
