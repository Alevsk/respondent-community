package domain

import "time"

// User represents a system user
type User struct {
	ID          string
	Email       string
	DisplayName string
	CreatedAt   time.Time
}
