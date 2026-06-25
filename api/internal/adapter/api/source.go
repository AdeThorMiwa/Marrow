package api

import (
	model "marrow/internal/model"
)

type SourceAdapter interface {
	Id() string

	Name() string

	// Retrieve source by identify, this is also use to check if an identify for a given source is valid
	// returns a source config for valid identifiers and error otherwise
	Resolve(identifier string) (model.SourceConfig, error)

	// given a source,
	FetchContents(source model.SourceConfig, limit int) ([]model.RawContent, error)
}
