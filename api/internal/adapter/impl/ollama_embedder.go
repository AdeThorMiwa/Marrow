package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	api "marrow/internal/adapter/api"
)

// ollamaClusteringPrefix is the task-conditioning prefix nomic-embed-text
// expects. Every call is forced into "clustering" mode — never
// "search_query"/"search_document" — because Enrichment's consumers (Feed
// cluster detection, Rabbithole similarity, Rabbithole centroid averaging)
// all need symmetric document-to-document similarity in one consistent
// embedding subspace. This is enforced here, not left to caller discipline.
const ollamaClusteringPrefix = "clustering: "

// chunkWordLimit is a conservative per-request word budget.
//
// nomic-embed-text advertises an 8192-token context, and its own Modelfile
// (confirmed via `ollama show nomic-embed-text --modelfile`) declares
// `PARAMETER num_ctx 8192` — but on a real running Ollama 0.21.0 instance,
// `ollama show nomic-embed-text` still reports "context length 2048", and
// requests above ~2000 words fail with "the input length exceeds the
// context length" regardless of also passing options.num_ctx=8192 or
// truncate=true on the request, or rebuilding the model via a custom
// Modelfile with the same PARAMETER already present. This is a confirmed
// upstream Ollama bug (ollama/ollama#7741 — "num_ctx does not increase
// context length above 2048"), not a gap in how this client calls the API.
// Real podcast-length transcripts exceed 2000 words routinely, so Embed
// chunks and mean-pools rather than trusting the model's advertised limit.
const chunkWordLimit = 1200

type OllamaEmbedder struct {
	BaseURL string
	Client  *http.Client
}

func NewOllamaEmbedder(baseURL string) *OllamaEmbedder {
	return &OllamaEmbedder{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
}

type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// Embed chunks text into chunkWordLimit-word pieces when it's long enough
// to risk exceeding Ollama's effective context limit, embeds each chunk
// independently (each still in "clustering" mode), and mean-pools the
// resulting vectors into one. Callers never see chunking — Embed always
// returns exactly one vector for the whole input, same as before.
func (o *OllamaEmbedder) Embed(ctx context.Context, text string, model api.EmbeddingModel) (*api.EmbeddingResponse, error) {
	chunks := chunkByWords(text, chunkWordLimit)

	sum := make([]float32, 0)
	for _, chunk := range chunks {
		vec, err := o.embedChunk(ctx, chunk, model)
		if err != nil {
			return nil, err
		}
		if len(sum) == 0 {
			sum = make([]float32, len(vec))
		}
		for i, v := range vec {
			sum[i] += v
		}
	}

	for i := range sum {
		sum[i] /= float32(len(chunks))
	}

	return &api.EmbeddingResponse{Vector: sum, Model: string(model)}, nil
}

func (o *OllamaEmbedder) embedChunk(ctx context.Context, text string, model api.EmbeddingModel) ([]float32, error) {
	reqBody, err := json.Marshal(ollamaEmbedRequest{
		Model: string(model),
		Input: ollamaClusteringPrefix + text,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode ollama embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/embed", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to build ollama embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call ollama embed endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embed endpoint returned status %d: %s", resp.StatusCode, body)
	}

	var parsed ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode ollama embed response: %w", err)
	}
	if len(parsed.Embeddings) == 0 {
		return nil, fmt.Errorf("ollama embed response contained no embeddings")
	}

	return parsed.Embeddings[0], nil
}

// maxWordLength discards whitespace-delimited "words" longer than this
// before chunking. Primary defense against this is now htmlToMarkdown
// (adapter/impl/html.go) — the real bug was raw HTML/attribute soup
// leaking into stored content, since fixed at the source for Substack and
// RSS-media. This stays as cheap insurance: no legitimate natural-language
// word is ever 100+ characters, but a legitimate long URL inside clean
// Markdown link/image syntax still could be — word-count-based chunking
// alone is blind to that either way.
const maxWordLength = 100

// chunkByWords splits text on whitespace into groups of at most limit
// words, discarding anomalously long tokens first (maxWordLength), always
// returning at least one chunk (even for empty text).
func chunkByWords(text string, limit int) []string {
	fields := strings.Fields(text)
	words := make([]string, 0, len(fields))
	for _, w := range fields {
		if len(w) <= maxWordLength {
			words = append(words, w)
		}
	}
	if len(words) == 0 {
		return []string{text}
	}

	var chunks []string
	for i := 0; i < len(words); i += limit {
		end := min(i+limit, len(words))
		chunks = append(chunks, strings.Join(words[i:end], " "))
	}
	return chunks
}
