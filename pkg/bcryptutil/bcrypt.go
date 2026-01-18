package bcryptutil

import (
	"golang.org/x/crypto/bcrypt"
)

// BcryptUtils defines the interface for password hashing and verification.
type BcryptUtils interface {
	// GenerateHash returns the bcrypt hash of the password.
	GenerateHash(s string) (string, error)
	// CompareHash compares a bcrypt hash with a password. Returns true if they match.
	CompareHash(s string, hash string) bool
}

// BcryptUtilsImpl is a concrete implementation of BcryptUtils.
type BcryptUtilsImpl struct{}

// GenerateHash generates a bcrypt hash from the given string.
func (b *BcryptUtilsImpl) GenerateHash(s string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CompareHash compares a plain text string with a stored hash.
func (b *BcryptUtilsImpl) CompareHash(s string, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(s))
	return err == nil
}
