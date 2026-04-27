package domain

import "context"

type FetchRequest struct {
	Limit int
}

type RawItem struct {
	Source      string
	Title       string
	URL         string
	Author      string
	PublishedAt string
	Content     string
	Tags        []string
	Language    string
}

type Source interface {
	Name() string
	Kind() string
	Fetch(ctx context.Context, req FetchRequest) ([]RawItem, error)
}
