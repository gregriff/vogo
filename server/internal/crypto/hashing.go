package crypto

import "golang.org/x/crypto/bcrypt"

func CompareHashAndPassword(hashed, plaintext string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plaintext))
}

func HashPassword(plaintext string) (hashedPassword string, err error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	return string(hashed), err
}
