package domain

import "time"

type BlogPost struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Slug           string    `json:"slug"`
	Excerpt        string    `json:"excerpt"`
	Content        string    `json:"content"`
	Author         string    `json:"author"`
	Tags           []string  `json:"tags,omitempty"`
	SourceName     string    `json:"source_name"`
	SourceURL      string    `json:"source_url"`
	SourceAt       time.Time `json:"source_at"`
	Language       string    `json:"language,omitempty"`
	SiteName       string    `json:"site_name"`
	IdempotencyKey string    `json:"idempotency_key"`
}
