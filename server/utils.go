package server

import "github.com/google/uuid"

// GenerateTaskID generates a unique task ID using UUID v4
func GenerateTaskID() string {
	return uuid.New().String()
}
