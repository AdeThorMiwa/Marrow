package services

import (
	"errors"
	api "marrow/internal/adapter/api"
	"marrow/internal/adapter/registry"
	model "marrow/internal/model"
	"reflect"
	"testing"
)

// fakeRateLimitedAdapter only reacts to its own dedicated fake identifier
// (rateLimitTestIdentifier) — every other Resolve call gets a plain,
// unrelated error, so registering it never affects any other test's real
// URLs. See docs/twitter-rate-limit-handling/design.md §7.
const rateLimitTestIdentifier = "ratelimit-test://fake"

type fakeRateLimitedAdapter struct{}

func (fakeRateLimitedAdapter) Id() string   { return "fake-rate-limited" }
func (fakeRateLimitedAdapter) Name() string { return "Fake Rate Limited" }
func (fakeRateLimitedAdapter) Resolve(identifier string) ([]model.SourceConfig, error) {
	if identifier == rateLimitTestIdentifier {
		return nil, api.ErrRateLimited
	}
	return nil, errors.New("fakeRateLimitedAdapter: not this one")
}
func (fakeRateLimitedAdapter) Verify(config model.SourceConfig) (model.SourceConfig, error) {
	return model.SourceConfig{}, errors.New("not implemented")
}
func (fakeRateLimitedAdapter) Discover(source model.SourceConfig, limit int) (api.DiscoverResult, error) {
	return api.DiscoverResult{}, errors.New("not implemented")
}

func init() {
	registry.Register(fakeRateLimitedAdapter{})
}

func TestResolveUrl_RateLimited_ShortCircuits(t *testing.T) {
	_, err := ResolveUrl(rateLimitTestIdentifier)
	if !errors.Is(err, api.ErrRateLimited) {
		t.Fatalf("expected ResolveUrl to propagate api.ErrRateLimited, got: %v", err)
	}
}

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

		if len(result) != 1 {
			t.Fatalf("expected exactly one candidate, got %d", len(result))
		}
		// StaleAfter and LogoURL are both computed from the live feed's
		// actual data (posting cadence, publication image) — real data,
		// not something to pin down with an exact expected value.
		got := result[0]
		got.StaleAfter = 0
		got.LogoURL = ""
		if !reflect.DeepEqual(got, source) {
			t.Errorf("ResolveURL() failed\ngot:  %+v\nwant: %+v", got, source)
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

		if !result.Reachable {
			t.Fatal("expected source to be reachable")
		}

		if result.Items == nil {
			t.Fatal("expected a slice of content, got nil")
		}

		if len(result.Items) != FetchLimit {
			t.Errorf("TestFetchContents() failed\nexpected_len:  %+v\ngot: %+v", FetchLimit, len(result.Items))
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
