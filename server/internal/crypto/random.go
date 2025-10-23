package crypto

import (
	"crypto/rand"
	"math/big"
)

const (
	InviteCodeLength     = 6
	UsernameSuffixLength = 4
)

// GenerateUsernameSuffix creates a suffix to append to all usernames to reasonably
// ensure no name collisions, considering the small target userbase
func GenerateUsernameSuffix() string {
	return "#" + secureRandomString(UsernameSuffixLength)
}

// GenerateInviteCode creates an invite code to be used by one client during registration. At time of registration,
// server should check that this invite code has not already been used
func GenerateInviteCode() string {
	return secureRandomString(InviteCodeLength)
}

func secureRandomString(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[n.Int64()]
	}
	return string(result)
}
