package vogo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type NewUser struct {
	Username,
	Password string
	InviteCode string
}

type RegistrationResponse struct {
	Username,
	FriendCode string
}

// Register asks the vogo-server to create a new user given the provided credentials and returns
// the official username and friend code if sucessful. It will exit if an error is encountered.
func Register(client http.Client, username, password, inviteCode string) (string, string, error) {
	newUser := NewUser{Username: username, Password: password, InviteCode: inviteCode}
	payload, err := json.Marshal(newUser)
	if err != nil {
		panic(err)
	}

	res, err := client.Post(
		"/register",
		"application/json; charset=utf-8",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", "", fmt.Errorf("request error: %w", err)
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return "", "", fmt.Errorf("request failed: %s", string(body))
	}

	// var finalUsername string
	resData := RegistrationResponse{}
	if err := json.NewDecoder(res.Body).Decode(&resData); err != nil {
		panic(err)
	}
	return resData.Username, resData.FriendCode, nil
}
