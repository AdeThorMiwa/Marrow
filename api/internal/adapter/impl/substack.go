package adapter

import (
	"context"
	"fmt"
	model "marrow/internal/model"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

type SubstackSourceAdapter struct {
	id     string
	name   string
	parser *gofeed.Parser
}

func NewSubstackAdapter() *SubstackSourceAdapter {
	return &SubstackSourceAdapter{
		id:     "substack",
		name:   "Substack",
		parser: gofeed.NewParser(),
	}
}

func (a *SubstackSourceAdapter) toRssURL(url string) string {
	url = strings.TrimSuffix(url, "/")
	if !strings.HasSuffix(url, "/feed") {
		return url + "/feed"
	}
	return url
}

func (a *SubstackSourceAdapter) Id() string {
	return a.id
}

func (a *SubstackSourceAdapter) Name() string {
	return a.name
}

func (a *SubstackSourceAdapter) Resolve(identifier string) (model.SourceConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rssURL := a.toRssURL(identifier)

	feed, err := a.parser.ParseURLWithContext(rssURL, ctx)
	if err != nil {
		return model.SourceConfig{}, fmt.Errorf("failed to resolve Substack publication: %w", err)
	}

	config := model.SourceConfig{
		Identifier: identifier,
		Name:       feed.Title,
		AdapterID:  a.id,
	}

	return config, nil
}

func (a *SubstackSourceAdapter) FetchContents(source model.SourceConfig, size int) ([]model.RawContent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rssURL := a.toRssURL(source.Identifier)

	feed, err := a.parser.ParseURLWithContext(rssURL, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Substack content: %w", err)
	}

	var contents []model.RawContent

	for i, item := range feed.Items {
		if i >= size {
			break
		}

		publishedAt := time.Now()
		if item.PublishedParsed != nil {
			publishedAt = *item.PublishedParsed
		}

		var coverImage string
		if item.Image != nil {
			coverImage = item.Image.URL
		}

		raw := model.RawContent{
			ID:             item.GUID,
			Title:          item.Title,
			Kind:           model.Text,
			Description:    item.Description,
			Contents:       []string{item.Content},
			URL:            item.Link,
			PublishedAt:    publishedAt,
			CoverImageUrls: []string{coverImage},
			Authors:        []model.Author{{ID: source.Identifier, Name: source.Name}},
			Metadata:       map[string]any{},
		}

		contents = append(contents, raw)
	}

	return contents, nil
}
