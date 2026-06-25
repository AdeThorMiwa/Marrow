package ingest

import (
	"fmt"
	api "marrow/internal/adapter/api"
	impl "marrow/internal/adapter/impl"

	model "marrow/internal/model"
)

var adapters = []api.SourceAdapter{
	impl.NewSubstackAdapter(),
}

func ResolveUrl(url string) (model.SourceConfig, error) {

	for _, adp := range adapters {
		config, err := adp.Resolve(url)

		if err == nil {
			return config, nil
		}

	}

	return model.SourceConfig{}, fmt.Errorf("no adapter found for URL: %s", url)
}

func FetchContents(config model.SourceConfig, limit int) ([]model.RawContent, error) {
	adapter, err := resolveAdapter(config.AdapterID)

	if err != nil {
		return nil, fmt.Errorf("failed to resolve adapter: %w", err)
	}

	contents, err := adapter.FetchContents(config, limit)

	if err != nil {
		return nil, fmt.Errorf("failed to prepare source runner: %w", err)
	}

	return contents, nil
}

func resolveAdapter(adapterID string) (api.SourceAdapter, error) {
	for _, adp := range adapters {
		if adp.Id() == adapterID {
			return adp, nil
		}
	}

	return nil, fmt.Errorf("no adapter found with id: %s", adapterID)
}
