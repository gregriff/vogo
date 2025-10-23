package vogo

import (
	"net/http"
	"time"

	"github.com/gregriff/vogo/cli/internal/services"
)

// NewClient provides an http.Client for miscellaneous requests to the vogo server
func NewClient(baseUrl, username, password string) *http.Client {
	vogoTransport := services.Transport{
		BaseURL:               baseUrl,
		Username:              username,
		Password:              password,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: &vogoTransport,
	}
}
