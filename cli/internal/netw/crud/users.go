package crud

// users.go implements user-related CRUD.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gregriff/vogo/shared/public"
	"github.com/gregriff/vogo/shared/requests"
	"github.com/gregriff/vogo/shared/responses"
)

// Register asks the vogo-server to create a new user given the provided credentials and returns
// the official username and friend code if successful. It will exit if an error is encountered.
func Register(ctx context.Context, client *http.Client, username, password, inviteCode string) (string, error) {
	data := requests.NewUser{Name: username, Password: password, InviteCode: inviteCode}
	payload, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("json marshal error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "post", "/register", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	res, err := client.Do(req) //nolint:gosec // G704: URL is static
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

// Status fetches friends, channels, and incoming calls.
func Status(ctx context.Context, client *http.Client) (status *responses.Status, err error) {
	req, err := http.NewRequestWithContext(ctx, "get", "/status", nil)
	if err != nil {
		err = fmt.Errorf("error creating request: %w", err)
		return
	}
	res, err := client.Do(req) //nolint:gosec // G704: URL is static
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

// AddFriend adds a friend. TODO: make this return a friend.
func AddFriend(ctx context.Context, client *http.Client, friendName string) (friend *public.User, err error) {
	data := requests.AddFriend{Name: friendName}
	payload, err := json.Marshal(data)
	if err != nil {
		err = fmt.Errorf("json marshal err")
		return friend, err
	}

	req, err := http.NewRequestWithContext(ctx, "post", "/friend", bytes.NewReader(payload))
	if err != nil {
		err = fmt.Errorf("error creating request: %w", err)
		return friend, err
	}
	res, err := client.Do(req) //nolint:gosec // G704: URL is static
	if err != nil {
		err = fmt.Errorf("request error: %w", err)
		return friend, err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		err = fmt.Errorf("request failed: %s", string(body))
		return friend, err
	}

	if err = json.NewDecoder(res.Body).Decode(&friend); err != nil {
		err = fmt.Errorf("json decode error: %w", err)
		return friend, err
	}
	return friend, err
}

// CreateChannel creates a persistent voice-chat channel.
func CreateChannel(ctx context.Context, client *http.Client, name, desc string, capacity int) (channel *public.Channel, err error) {
	data := requests.CreateChannel{Name: name, Description: desc, Capacity: capacity}
	payload, err := json.Marshal(data)
	if err != nil {
		err = fmt.Errorf("json marshal err")
		return channel, err
	}

	req, err := http.NewRequestWithContext(ctx, "post", "/channel", bytes.NewReader(payload))
	if err != nil {
		err = fmt.Errorf("error creating request: %w", err)
		return channel, err
	}
	res, err := client.Do(req) //nolint:gosec // G704: URL is static
	if err != nil {
		err = fmt.Errorf("request error: %w", err)
		return channel, err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		err = fmt.Errorf("request failed: %s", string(body))
		return channel, err
	}

	if err = json.NewDecoder(res.Body).Decode(&channel); err != nil {
		err = fmt.Errorf("json decode error: %w", err)
		return channel, err
	}
	return channel, err
}

// InviteFriend invites a friend to a channel owned by the user inviting.
func InviteFriend(ctx context.Context, client *http.Client, channelName, friendName string) (friend *public.User, err error) {
	data := requests.InviteFriend{ChannelName: channelName, FriendName: friendName}
	payload, err := json.Marshal(data)
	if err != nil {
		err = fmt.Errorf("json marshal err")
		return friend, err
	}

	req, err := http.NewRequestWithContext(ctx, "post", "/channel/invite", bytes.NewReader(payload))
	if err != nil {
		err = fmt.Errorf("error creating request: %w", err)
		return friend, err
	}
	res, err := client.Do(req) //nolint:gosec // G704: URL is static
	if err != nil {
		err = fmt.Errorf("request error: %w", err)
		return friend, err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		err = fmt.Errorf("request failed: %s", string(body))
		return friend, err
	}

	if err = json.NewDecoder(res.Body).Decode(&friend); err != nil {
		err = fmt.Errorf("json decode error: %w", err)
		return friend, err
	}
	return friend, err
}
