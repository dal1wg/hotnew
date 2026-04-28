package domain

import "time"

type Article struct {
	ID          string    `json:"id"`
	Source      string    `json:"source"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Author      string    `json:"author,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	Content     string    `json:"content,omitempty"`
	Summary     string    `json:"summary,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Language    string    `json:"language,omitempty"`
}
