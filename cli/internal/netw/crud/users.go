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
	Name,
	Password string
	InviteCode string
}

// Register asks the vogo-server to create a new user given the provided credentials and returns
// the official username and friend code if sucessful. It will exit if an error is encountered.
func Register(client *http.Client, username, password, inviteCode string) (string, error) {
	newUser := newUser{Name: username, Password: password, InviteCode: inviteCode}
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
	Name string
}

type Friend struct {
	user
	Status string
}

type Channel struct {
	Owner,
	Name,
	Description string

	Capacity int

	MemberNames string
}

type statusResponse struct {
	Friends  []Friend
	Channels []Channel
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

// AddFriend adds a friend. TODO: make this return a friend
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

// CreateChannel creates a persistent voice-chat channel.
func CreateChannel(client *http.Client, name, desc string, cap int) (channel *Channel, err error) {
	req := struct {
		Name,
		Description string
		Capacity int
	}{Name: name}

	payload, err := json.Marshal(req)
	if err != nil {
		err = fmt.Errorf("json marshal err")
		return
	}

	res, err := client.Post("/channel",
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

	if err = json.NewDecoder(res.Body).Decode(&channel); err != nil {
		err = fmt.Errorf("json decode error: %w", err)
		return
	}
	return
}

type inviteFriendResponse struct {
	Name string
}

func InviteFriend(client *http.Client, channelName, friendName string) (friend *inviteFriendResponse, err error) {
	req := struct {
		ChannelName,
		FriendName string
	}{ChannelName: channelName, FriendName: friendName}

	payload, err := json.Marshal(req)
	if err != nil {
		err = fmt.Errorf("json marshal err")
		return
	}

	res, err := client.Post("/invite",
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

	if err = json.NewDecoder(res.Body).Decode(&friend); err != nil {
		err = fmt.Errorf("json decode error: %w", err)
		return
	}
	return
}
