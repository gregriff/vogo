// package signaling contains code needed to perform signaling with the vogo server during the webrtc connection process.
// It is seperate from the vogo service because eventually this will be done via websockets and behavior will be sufficiently different.
package signaling

import (
	"net/http"
	"time"

	"github.com/gregriff/vogo/cli/internal/services"
)

// NewClient provides an http.Client for signaling requests
func NewClient(baseUrl, username, password string) *http.Client {
	signalingTransport := services.Transport{
		BaseURL:               baseUrl,
		Username:              username,
		Password:              password,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	return &http.Client{
		Timeout:   120 * time.Second,
		Transport: &signalingTransport,
	}
}
