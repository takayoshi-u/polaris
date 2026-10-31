package id

import "github.com/google/uuid"

// NewV7 generates a new UUID v7.
func NewV7() uuid.UUID {
	u, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}
	return u
}

// NewV7String generates a new UUID v7 as a string.
func NewV7String() string {
	return NewV7().String()
}

// IsValid checks if a string is a valid UUID.
func IsValid(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

