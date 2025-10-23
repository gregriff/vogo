package services

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// Transport allows custom attributes to be added to each HTTP request sent by an http.Client that uses this transport
type Transport struct {
	BaseURL,
	Username,
	Password string
	MaxIdleConns int
	IdleConnTimeout,
	TLSHandshakeTimeout,
	ResponseHeaderTimeout time.Duration
}

// RoundTrip adds upon the normal http.Transport.RoundTrip() behavior to add basic auth and a base url to each request.
// Reference: https://cs.opensource.google/go/x/oauth2/+/refs/tags/v0.31.0:transport.go
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()

	baseURL := strings.TrimSuffix(t.BaseURL, "/")
	path := "/" + strings.TrimPrefix(url, "/")
	newURL, err := req.URL.Parse(baseURL + path)
	if err != nil {
		log.Fatalf("URL PARSE ERROR: %v", err)
	}
	req.URL = newURL
	log.Println("making request to vogo server: ", req.Proto, url)

	if path != "/register" {
		log.Println("setting basic auth: ", t.Username, t.Password)
		req.SetBasicAuth(t.Username, t.Password)
	}
	return http.DefaultTransport.RoundTrip(req)
}
