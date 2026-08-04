package ingest

import (
	"fmt"

	api "marrow/internal/adapter/api"
	"marrow/internal/adapter/registry"
	model "marrow/internal/model"
)

func ResolveUrl(url string) (model.SourceConfig, error) {
	for _, adp := range registry.SourceAdapters() {
		config, err := adp.Resolve(url)
		if err == nil {
			return config, nil
		}
	}

	return model.SourceConfig{}, fmt.Errorf("no adapter found for URL: %s", url)
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
