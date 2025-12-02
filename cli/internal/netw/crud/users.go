package crud

// users.go implements user-related CRUD.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type newUser struct {
	name,
	password string
	inviteCode string
}

// Register asks the vogo-server to create a new user given the provided credentials and returns
// the official username and friend code if sucessful. It will exit if an error is encountered.
func Register(client *http.Client, username, password, inviteCode string) (string, error) {
	newUser := newUser{name: username, password: password, inviteCode: inviteCode}
	payload, err := json.Marshal(newUser)
	if err != nil {
		return "", fmt.Errorf("json marshal error: %w", err)
	}

	res, err := client.Post(
		"/register",
		"application/json; charset=utf-8",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", fmt.Errorf("request error: %w", err)
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("request failed: %s", string(body))
	}

	if err := json.NewDecoder(res.Body).Decode(&username); err != nil {
		return "", fmt.Errorf("json decode error: %w", err)
	}
	return username, nil
}

type user struct {
	name string
}

type channel struct {
	owner,
	name,
	description string
	capacity int

	members []string
}

type statusResponse struct {
	friends  []user
	channels []channel
}

// Status fetches friends, channels, and incoming calls.
func Status(client *http.Client) (status *statusResponse, err error) {
	res, err := client.Get("/status")
	if err != nil {
		err = fmt.Errorf("request error: %w", err)
		return
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		err = fmt.Errorf("request failed: %s", string(body))
		return
	}

	if err = json.NewDecoder(res.Body).Decode(&status); err != nil {
		err = fmt.Errorf("json decode error: %w", err)
		return
	}
	return
}

type addFriendResponse struct {
	Name string
}

// AddFriend adds a friend
func AddFriend(client *http.Client, friendName string) (status *addFriendResponse, err error) {
	req := struct {
		Name string
	}{Name: friendName}

	payload, err := json.Marshal(req)
	if err != nil {
		err = fmt.Errorf("json marshal err")
		return
	}

	res, err := client.Post("/friend",
		"application/json; charset=utf-8",
		bytes.NewReader(payload),
	)
	if err != nil {
		err = fmt.Errorf("request error: %w", err)
		return
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		err = fmt.Errorf("request failed: %s", string(body))
		return
	}

	if err = json.NewDecoder(res.Body).Decode(&status); err != nil {
		err = fmt.Errorf("json decode error: %w", err)
		return
	}
	return
}
