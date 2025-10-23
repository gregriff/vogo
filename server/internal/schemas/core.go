package schemas

import (
	"time"

	"github.com/google/uuid"
)

// User stores information about a vogo client
type User struct {
	// for DB storage, never changes. Not given to anyone
	Id uuid.UUID

	// public username. should be unique (small groups), but might change
	// format: [name]#XX
	Name string

	// hashed password
	Password string

	CreatedAt time.Time
}

type InviteCode struct {
	Id               uuid.UUID
	Code             string
	RegisteredUserId uuid.UUID
	createdAt        time.Time
}
