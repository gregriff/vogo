// package crud implements RESTful requests to the vogo server
package crud

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// NewClient provides an http.Client for miscellaneous requests to the vogo server.
func NewClient(baseUrl, username, password string) *http.Client {
	vogoTransport := transport{
		baseUrl:               baseUrl,
		username:              username,
		password:              password,
		maxIdleConns:          10,
		idleConnTimeout:       30 * time.Second,
		tlsHandshakeTimeout:   5 * time.Second,
		responseHeaderTimeout: 10 * time.Second,
	}

	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: &vogoTransport,
	}
}

// transport allows custom attributes to be added to each HTTP request sent by an http.Client that uses this transport.
type transport struct {
	baseUrl,
	username,
	password string
	maxIdleConns int
	idleConnTimeout,
	tlsHandshakeTimeout,
	responseHeaderTimeout time.Duration
}

// RoundTrip adds upon the normal http.Transport.RoundTrip() behavior to add basic auth and a base url to each request.
// Reference: https://cs.opensource.google/go/x/oauth2/+/refs/tags/v0.31.0:transport.go
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()

	baseURL := strings.TrimSuffix(t.baseUrl, "/")
	path := "/" + strings.TrimPrefix(url, "/")
	newURL, err := req.URL.Parse(baseURL + path)
	if err != nil {
		log.Fatalf("URL PARSE ERROR: %v", err)
	}
	req.URL = newURL
	log.Println("making request to vogo server: ", req.Proto, url) //nolint:gosec // G704: URL is app-generated

	if path != "/register" {
		req.SetBasicAuth(t.username, t.password)
	}
	return http.DefaultTransport.RoundTrip(req)
}
