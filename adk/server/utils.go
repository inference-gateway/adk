package server

import "github.com/google/uuid"

// StringPtr returns a pointer to the given string
func StringPtr(s string) *string {
	return &s
}

// GenerateTaskID generates a unique task ID using UUID v4
func GenerateTaskID() string {
	return uuid.New().String()
}
