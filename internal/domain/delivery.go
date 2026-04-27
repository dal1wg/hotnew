package domain

import "time"

type DeliveryRecord struct {
	ID          string    `json:"id"`
	ArticleID   string    `json:"article_id"`
	Channel     string    `json:"channel"`
	Target      string    `json:"target"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	AttemptedAt time.Time `json:"attempted_at"`
}
