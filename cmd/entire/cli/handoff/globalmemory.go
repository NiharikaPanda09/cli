package handoff

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/entireio/cli/redact"
)

const (
	SectionGlobalMemory = "prior_art"

	vectorQueryTimeout = 20 * time.Second
	vectorNumResults   = 5
	maxQueryTextChars  = 400
)

type VectorSearcher interface {
	Query(ctx context.Context, text string, numResults int) ([]PriorHit, error)
}

type PriorHit struct {
	RepoKey      string
	CheckpointID string
	SessionID    int
	SectionName  string
	ItemText     string
	CiteLine     int
	Score        float64
}

type VectorConfig struct {
	Host  string
	Token string
	Index string
}

func VectorConfigFromEnv() (VectorConfig, bool) {
	c := VectorConfig{
		Host:  strings.TrimSpace(os.Getenv("DATABRICKS_HOST")),
		Token: strings.TrimSpace(os.Getenv("DATABRICKS_TOKEN")),
		Index: strings.TrimSpace(os.Getenv("DATABRICKS_VECTOR_INDEX")),
	}
	if c.Host == "" || c.Token == "" || c.Index == "" {
		return c, false
	}
	if !strings.HasPrefix(c.Host, "https://") {
		return c, false
	}
	return c, true
}

type restVectorSearcher struct {
	cfg    VectorConfig
	client *http.Client
}

func NewVectorSearcher(cfg VectorConfig) VectorSearcher {
	return &restVectorSearcher{cfg: cfg, client: &http.Client{Timeout: vectorQueryTimeout}}
}

type vectorQueryRequest struct {
	Columns     []string `json:"columns"`
	QueryText   string   `json:"query_text"`
	NumResults  int      `json:"num_results"`
	FiltersJSON string   `json:"filters_json,omitempty"`
}

type vectorQueryResponse struct {
	Manifest struct {
		Columns []struct {
			Name string `json:"name"`
		} `json:"columns"`
	} `json:"manifest"`
	Result struct {
		DataArray [][]any `json:"data_array"`
	} `json:"result"`
}

var priorArtColumns = []string{
	"repo_key", "checkpoint_id", "session_id", "section_name", "item_text", "cite_line",
}

func (s *restVectorSearcher) Query(ctx context.Context, text string, numResults int) ([]PriorHit, error) {
	body, err := json.Marshal(vectorQueryRequest{
		Columns:     priorArtColumns,
		QueryText:   text,
		NumResults:  numResults,
		FiltersJSON: `{"section_name": "dead_ends"}`,
	})
	if err != nil {
		return nil, fmt.Errorf("encode vector query: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/2.0/vector-search/indexes/%s/query",
		strings.TrimSuffix(s.cfg.Host, "/"), s.cfg.Index)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build vector query: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vector query: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("vector query: %s", resp.Status)
	}

	var out vectorQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode vector response: %w", err)
	}
	return hitsFrom(out), nil
}

func hitsFrom(out vectorQueryResponse) []PriorHit {
	idx := make(map[string]int, len(out.Manifest.Columns))
	for i, c := range out.Manifest.Columns {
		idx[c.Name] = i
	}

	hits := make([]PriorHit, 0, len(out.Result.DataArray))
	for _, row := range out.Result.DataArray {
		get := func(name string) any {
			i, ok := idx[name]
			if !ok || i >= len(row) {
				return nil
			}
			return row[i]
		}
		h := PriorHit{
			RepoKey:      asString(get("repo_key")),
			CheckpointID: asString(get("checkpoint_id")),
			SessionID:    asInt(get("session_id")),
			SectionName:  asString(get("section_name")),
			ItemText:     asString(get("item_text")),
			CiteLine:     asInt(get("cite_line")),
		}
		if h.ItemText == "" || h.CheckpointID == "" {
			continue
		}
		hits = append(hits, h)
	}
	return hits
}

func asString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func asInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case string:
		var out int
		if _, err := fmt.Sscanf(n, "%d", &out); err == nil {
			return out
		}
	}
	return 0
}

type globalMemoryExtractor struct {
	searcher VectorSearcher
	ask      string
}

func newGlobalMemoryExtractor(s VectorSearcher, ask string) Extractor {
	return globalMemoryExtractor{searcher: s, ask: ask}
}

func (globalMemoryExtractor) Name() string { return SectionGlobalMemory }

func (g globalMemoryExtractor) Extract(ctx context.Context, in Input) (Section, error) {
	if g.searcher == nil {
		return section(SectionGlobalMemory, nil, "vector search not configured"), nil
	}

	query := strings.TrimSpace(g.ask)
	if query == "" {
		query = queryTextFrom(in)
	}
	if query == "" {
		return section(SectionGlobalMemory, nil, "nothing to search on: no intent in range and no --ask given"), nil
	}

	// Privacy Boundary: query text originates from either --ask (typed
	// verbatim by the user) or a checkpoint's Summary.Intent -- both are
	// prompt-shaped text. Neither has been through the checkpoint storage
	// redaction pipeline by this point, and this is the one place in the
	// package that leaves the process over the network, to a service Entire
	// does not control. Redact before it is ever placed in an HTTP request
	// body, using only the local, non-network scanners (redact.String makes
	// no outbound call itself).
	query = redact.String(query)

	hits, err := g.searcher.Query(ctx, truncate(query, maxQueryTextChars), vectorNumResults)
	if err != nil {
		return section(SectionGlobalMemory, nil, fmt.Sprintf("vector search unavailable: %v", err)), nil
	}
	if len(hits) == 0 {
		return section(SectionGlobalMemory, nil, "no prior work found for this task"), nil
	}

	items := make([]Item, 0, len(hits))
	for _, h := range hits {
		items = append(items, Item{
			Text: describeHit(h, in.Repo),
			Cites: []Citation{{
				CheckpointID: h.CheckpointID,
				SessionIndex: h.SessionID,
				Line:         h.CiteLine,
			}},
		})
	}
	return Section{
		Name:  SectionGlobalMemory,
		Items: items,
		Note:  "retrieved by similarity across the team's history; verify before relying on it",
	}, nil
}

func describeHit(h PriorHit, currentRepo string) string {
	if h.RepoKey != "" && h.RepoKey != currentRepo {
		return fmt.Sprintf("[%s] %s", h.RepoKey, h.ItemText)
	}
	return h.ItemText
}

func queryTextFrom(in Input) string {
	var parts []string
	for _, cp := range in.Checkpoints {
		if cp.Summary == nil {
			continue
		}
		if s := strings.TrimSpace(cp.Summary.Intent); s != "" {
			parts = append(parts, s)
		}
		if len(parts) >= 2 {
			break
		}
	}
	return strings.Join(parts, " ")
}
