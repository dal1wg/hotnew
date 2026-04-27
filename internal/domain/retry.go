package domain

import "time"

type RetryJob struct {
	ID            string    `json:"id"`
	ArticleID     string    `json:"article_id"`
	Channel       string    `json:"channel"`
	Target        string    `json:"target"`
	Status        string    `json:"status"`
	Attempts      int       `json:"attempts"`
	MaxAttempts   int       `json:"max_attempts"`
	LastError     string    `json:"last_error,omitempty"`
	NextAttemptAt time.Time `json:"next_attempt_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RetryFilter struct {
	Status    string
	Channel   string
	ArticleID string
	Limit     int
}
