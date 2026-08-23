package api

import "errors"

// ErrRateLimited: see docs/twitter-rate-limit-handling/design.md §4.
var ErrRateLimited = errors.New("adapter: rate limited")
