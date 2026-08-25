# Feed Source Diversity — Tasks

1. Add `MaxSameSourcePerPage` to `FeedConfig` (`internal/config.go`) and
   set it in `configs/base.yaml` (default `5`).
2. Implement `applyDiversityCap` in `internal/feed/content_source.go` and
   wire it into `ContentFeedSource.Produce` in place of the current
   plain trim.
3. Unit test `applyDiversityCap` directly: burst capped, normal set
   unchanged, `cap<=0` disables it.
4. Real-infra test on `ContentFeedSource.Produce`: burst source capped
   on page 1, page 2 (via returned cursor) resumes with no gap/dup.
5. Run `go build ./... && go vet ./...` and the `internal/feed` test
   package; confirm green.
