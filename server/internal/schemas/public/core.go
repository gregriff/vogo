// package public contains structs that can be sent to the vogo client.
// These structs do not contain private information such as passwords, or
// internal info such as UUIDs or timestamps. Structs used to represent database
// records and other server-specific objects should embed these public structs.
package public

// User stores information about the user of a vogo client
type User struct {
	// public username. should be unique (small groups), but might change
	// format: [name]#XX
	Name string
}

// Channel represents a chat room of multiple Users. It is created
// by a User, and any member can invite new Users.
type Channel struct {
	Owner,
	Name,
	Description string

	// Defaults to 6 (db enforced), due to WebRTC limitations
	Capacity int

	MemberNames []string
}
