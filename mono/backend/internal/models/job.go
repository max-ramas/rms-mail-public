package models

import "time"

type Job struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Payload   string    `json:"payload"`
	Status    string    `json:"status"` // pending, processing, completed, failed
	Attempt   int       `json:"attempt"`
	NextRunAt time.Time `json:"next_run_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
