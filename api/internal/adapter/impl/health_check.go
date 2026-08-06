package adapter

import (
	"context"
	"net/http"
	"time"
)

// assertServiceUp does a best-effort GET against baseURL's root and panics
// if it doesn't come back — shared by every adapter constructor that wraps
// a local, self-hosted service (Ollama, whisper.cpp) rather than a public
// API, where "down" almost always means "forgot to start it locally," not a
// transient blip worth tolerating at boot. Any response at all (status
// aside) counts as "up" — this only needs to catch connection failures
// (nothing listening on that port), not validate the response shape.
func assertServiceUp(name, baseURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		panic(name + " health check failed: invalid base URL " + baseURL + ": " + err.Error())
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(name + " is not reachable at " + baseURL + " — start it before running marrow: " + err.Error())
	}
	resp.Body.Close()
}
