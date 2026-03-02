package main

import "time"

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusProcessing TaskStatus = "processing"
	StatusDone       TaskStatus = "done"
	StatusFailed     TaskStatus = "failed"
)

type Task struct {
	ID           string     `json:"id"`
	Status       TaskStatus `json:"status"`
	HTML         string     `json:"html,omitempty"`
	Preview      []byte     `json:"-"`
	TaskPassword string     `json:"-"`
	Error        string     `json:"error,omitempty"`
	InputTokens  int        `json:"input_tokens,omitempty"`
	OutputTokens int        `json:"output_tokens,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
