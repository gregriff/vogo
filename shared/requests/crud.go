// package requests contains structs of http request bodies,
// sent by the client to the server.
package requests

// NewUser is the request data to register a new client with the server.
type NewUser struct {
	Name,
	Password,
	InviteCode string
}

type AddFriend struct {
	Name string
}

type InviteFriend struct {
	ChannelName,
	FriendName string
}

type CreateChannel struct {
	Name,
	Description string
	Capacity int
}
