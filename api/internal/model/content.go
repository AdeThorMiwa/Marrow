package models

import "time"

type ContentKind string

const (
	Text  ContentKind = "text"
	Audio ContentKind = "audio"
	Video ContentKind = "video"
)

type Author struct {
	ID   string
	Name string
	Url  string
}

type RawContent struct {
	ID             string
	Title          string
	Kind           ContentKind
	CoverImageUrls []string
	Description    string
	Contents       []string
	URL            string
	PublishedAt    time.Time
	MediaRef       string
	Metadata       map[string]any
	Authors        []Author
}
