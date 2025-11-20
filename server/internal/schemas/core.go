package schemas

import (
	"time"

	"github.com/google/uuid"
	"github.com/gregriff/vogo/server/internal/schemas/public"
)

// User is the database representation of public.User, without the password column.
type User struct {
	public.User

	Id        uuid.UUID
	CreatedAt time.Time
}

// UserWithPassword is the full database representation of User
type UserWithPassword struct {
	User

	// hashed password
	Password string
}

// Channel is the database representation of public.Channel
type Channel struct {
	public.Channel

	Id        uuid.UUID
	CreatedAt time.Time
}
