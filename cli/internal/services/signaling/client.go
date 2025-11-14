// package signaling contains code needed to perform signaling with the vogo server during the webrtc connection process.
// It is seperate from the vogo service because eventually this will be done via websockets and behavior will be sufficiently different.
package signaling

import (
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gregriff/vogo/cli/internal/services"
	"golang.org/x/net/websocket"
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

// NewWsConfig creates a new websocket.Config for the vogo server for a specific endpoint, with basic auth.
func NewWsConfig(baseUrl, username, password, endpoint string) (*websocket.Config, error) {
	url := strings.Replace(baseUrl, "http", "ws", 1) + endpoint
	log.Println("ws url: url")
	cfg, err := websocket.NewConfig(url, "") // no origin b/c we're not a browser
	if err != nil {
		return nil, err
	}

	// set basic auth for the http request that initates the ws connection
	auth := username + ":" + password
	auth = base64.StdEncoding.EncodeToString([]byte(auth))
	cfg.Header.Set("Authorization", "Basic "+auth)

	return cfg, nil
}
