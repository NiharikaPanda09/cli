package handoff

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apicheckpoint "github.com/entireio/cli/api/checkpoint"
)

const priorCheckpointID = "01PRIOR"

type fakeSearcher struct {
	hits    []PriorHit
	err     error
	gotText string
}

func (f *fakeSearcher) Query(_ context.Context, text string, _ int) ([]PriorHit, error) {
	f.gotText = text
	return f.hits, f.err
}

func intentInput() Input {
	return Input{
		Repo: "gh/acme/cli",
		Checkpoints: []Checkpoint{{
			ID:       testCheckpointID,
			Metadata: &apicheckpoint.Metadata{},
			Summary:  &apicheckpoint.Summary{Intent: "add a --json flag to recap"},
		}},
	}
}

func TestGlobalMemoryIsAbsentUnlessConfigured(t *testing.T) {
	t.Parallel()

	p := Build(context.Background(), intentInput(), true)
	for _, sec := range p.Sections {
		if sec.Name == SectionGlobalMemory {
			t.Fatal("prior_art must not appear when vector search is unconfigured")
		}
	}
	if len(p.Sections) != 4 {
		t.Errorf("want 4 sections without graph or search, got %d", len(p.Sections))
	}
}

func TestGlobalMemoryAppearsWhenSearcherPresent(t *testing.T) {
	t.Parallel()

	s := &fakeSearcher{hits: []PriorHit{{
		RepoKey: "gh/acme/other", CheckpointID: priorCheckpointID, SessionID: 0,
		SectionName: "dead_ends", ItemText: "go test -run X — failed: undefined", CiteLine: 42,
	}}}
	p := BuildWith(context.Background(), intentInput(), BuildOptions{NoGraph: true, Searcher: s})

	var found *Section
	for i := range p.Sections {
		if p.Sections[i].Name == SectionGlobalMemory {
			found = &p.Sections[i]
		}
	}
	if found == nil {
		t.Fatal("prior_art section missing")
	}
	if len(found.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(found.Items))
	}
	if !strings.Contains(found.Items[0].Text, "gh/acme/other") {
		t.Errorf("a hit from another repo must be labelled: %q", found.Items[0].Text)
	}
}

func TestGlobalMemoryPreservesCitationsFromRetrieval(t *testing.T) {
	t.Parallel()

	s := &fakeSearcher{hits: []PriorHit{{
		RepoKey: "gh/acme/cli", CheckpointID: priorCheckpointID, SessionID: 2,
		ItemText: "boom", CiteLine: 412,
	}}}
	sec, err := newGlobalMemoryExtractor(s, "").Extract(context.Background(), intentInput())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	c := sec.Items[0].Cites[0]
	if c.CheckpointID != priorCheckpointID || c.SessionIndex != 2 || c.Line != 412 {
		t.Errorf("retrieval lost provenance: %+v", c)
	}
}

func TestGlobalMemoryUsesIntentAsQueryByDefault(t *testing.T) {
	t.Parallel()

	s := &fakeSearcher{}
	if _, err := newGlobalMemoryExtractor(s, "").Extract(context.Background(), intentInput()); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(s.gotText, "--json flag") {
		t.Errorf("query text = %q, want the session intent", s.gotText)
	}
}

func TestGlobalMemoryAskOverridesIntent(t *testing.T) {
	t.Parallel()

	s := &fakeSearcher{}
	if _, err := newGlobalMemoryExtractor(s, "how do I mock the store").Extract(context.Background(), intentInput()); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if s.gotText != "how do I mock the store" {
		t.Errorf("--ask must win over intent, got %q", s.gotText)
	}
}

func TestGlobalMemoryFailureIsANote(t *testing.T) {
	t.Parallel()

	s := &fakeSearcher{err: errors.New("endpoint asleep")}
	sec, err := newGlobalMemoryExtractor(s, "").Extract(context.Background(), intentInput())
	if err != nil {
		t.Fatalf("a failing search must not fail the packet: %v", err)
	}
	if len(sec.Items) != 0 || !strings.Contains(sec.Note, "unavailable") {
		t.Errorf("want an unavailable note, got items=%d note=%q", len(sec.Items), sec.Note)
	}
}

func TestVectorSearcherPostsQueryTextAndBearer(t *testing.T) {
	t.Parallel()

	var gotAuth, gotPath string
	var gotBody vectorQueryRequest
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if derr := json.NewDecoder(r.Body).Decode(&gotBody); derr != nil {
			t.Errorf("decode: %v", derr)
		}
		if _, werr := w.Write([]byte(`{
			"manifest":{"columns":[{"name":"repo_key"},{"name":"checkpoint_id"},
				{"name":"session_id"},{"name":"section_name"},{"name":"item_text"},{"name":"cite_line"}]},
			"result":{"data_array":[["gh/a/b","01ABC",0,"dead_ends","it failed",99]]}}`)); werr != nil {
			t.Errorf("write: %v", werr)
		}
	}))
	defer srv.Close()

	s := &restVectorSearcher{
		cfg:    VectorConfig{Host: srv.URL, Token: "tok", Index: "main.default.idx"},
		client: srv.Client(),
	}
	hits, err := s.Query(context.Background(), "why does X fail", 5)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth = %q", gotAuth)
	}
	if !strings.Contains(gotPath, "/api/2.0/vector-search/indexes/main.default.idx/query") {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody.QueryText != "why does X fail" {
		t.Errorf("query_text = %q", gotBody.QueryText)
	}
	if len(hits) != 1 || hits[0].CheckpointID != "01ABC" || hits[0].CiteLine != 99 {
		t.Fatalf("hit not parsed: %+v", hits)
	}
}

func TestVectorSearcherDropsRowsWithoutProvenance(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, werr := w.Write([]byte(`{
			"manifest":{"columns":[{"name":"checkpoint_id"},{"name":"item_text"}]},
			"result":{"data_array":[["","orphan text"],["01OK","good text"]]}}`)); werr != nil {
			t.Errorf("write: %v", werr)
		}
	}))
	defer srv.Close()

	s := &restVectorSearcher{cfg: VectorConfig{Host: srv.URL, Token: "t", Index: "i"}, client: srv.Client()}
	hits, err := s.Query(context.Background(), "q", 5)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(hits) != 1 || hits[0].CheckpointID != "01OK" {
		t.Fatalf("an uncitable row must be dropped, got %+v", hits)
	}
}

func TestVectorConfigRequiresHTTPS(t *testing.T) {
	t.Setenv("DATABRICKS_HOST", "http://insecure.example.com")
	t.Setenv("DATABRICKS_TOKEN", "t")
	t.Setenv("DATABRICKS_VECTOR_INDEX", "i")
	if _, ok := VectorConfigFromEnv(); ok {
		t.Error("plain http must not configure a searcher")
	}
}
