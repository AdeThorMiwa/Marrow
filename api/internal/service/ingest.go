package services

import (
	"errors"
	"fmt"

	api "marrow/internal/adapter/api"
	"marrow/internal/adapter/registry"
	model "marrow/internal/model"
)

// ResolveUrl tries each registered adapter until one recognizes the
// identifier, returning its candidates — which may be empty (recognized
// but no publication found, e.g. an empty profile) even with a nil error.
//
// ErrRateLimited short-circuits: see
// docs/twitter-rate-limit-handling/design.md §7.
func ResolveUrl(url string) ([]model.SourceConfig, error) {
	for _, adp := range registry.SourceAdapters() {
		configs, err := adp.Resolve(url)
		if err == nil {
			return configs, nil
		}
		if errors.Is(err, api.ErrRateLimited) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("no adapter found for URL: %s", url)
}

func FetchContents(config model.SourceConfig, limit int) (api.DiscoverResult, error) {
	adapter, err := registry.SourceAdapter(config.AdapterID)
	if err != nil {
		return api.DiscoverResult{}, fmt.Errorf("failed to resolve adapter: %w", err)
	}

	result, err := adapter.Discover(config, limit)
	if err != nil {
		return api.DiscoverResult{}, fmt.Errorf("failed to prepare source runner: %w", err)
	}

	return result, nil
}
