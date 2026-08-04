package feed_test

import (
	lib "marrow/internal"
)

var testConfig = lib.Config{
	Feed: lib.FeedConfig{
		DefaultPageSize: 20,
		OverfetchFactor: 5,
		ChronologyDecay: 0.05,
	},
}
