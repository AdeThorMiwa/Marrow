package ingest

import (
	model "marrow/internal/model"
	"reflect"
	"testing"
)

var source = model.SourceConfig{
	Name:       "Perspectives",
	Identifier: "https://debliu.substack.com",
	AdapterID:  "substack",
}

const (
	FetchLimit = 2
)

func TestResolveUrl(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		url := "https://debliu.substack.com"

		result, err := ResolveUrl(url)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		expected := source

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("ResolveURL() failed\ngot:  %+v\nwant: %+v", result, expected)
		}
	})

	t.Run("failure", func(t *testing.T) {
		url := "https://completely-unsupported-domain.com"

		_, err := ResolveUrl(url)

		if err == nil {
			t.Errorf("expected an error for an invalid URL, but got nil")
		}
	})

}

func TestFetchContents(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		result, err := FetchContents(source, FetchLimit)

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if result == nil {
			t.Fatal("expected a slice of content, got nil")
		}

		if len(result) != FetchLimit {
			t.Errorf("TestFetchContents() failed\nexpected_len:  %+v\ngot: %+v", FetchLimit, len(result))
		}
	})

	t.Run("failure - invalid adapter id", func(t *testing.T) {

		config := model.SourceConfig{
			Identifier: "https://substack.com",
			AdapterID:  "invalid-adapter-id-here",
		}

		_, err := FetchContents(config, FetchLimit)

		if err == nil {
			t.Errorf("expected an error for an unregistered adapter, but got nil")
		}
	})

}
