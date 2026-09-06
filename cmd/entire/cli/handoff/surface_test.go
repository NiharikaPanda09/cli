package handoff

import (
	"context"
	"errors"
	"strings"
	"testing"

	apicheckpoint "github.com/entireio/cli/api/checkpoint"
)

const testFileA = "a.go"

type fakeGraph struct {
	out []byte
	err error
}

func (f fakeGraph) Run(context.Context, ...string) ([]byte, error) { return f.out, f.err }

func surfaceInput(g GraphRunner, files ...string) Input {
	return Input{
		Repo:  "r",
		Graph: g,
		Checkpoints: []Checkpoint{{
			ID:           testCheckpointID,
			Metadata:     &apicheckpoint.Metadata{},
			FilesTouched: files,
		}},
	}
}

func TestSurfaceRanksByDependents(t *testing.T) {
	t.Parallel()

	g := fakeGraph{out: []byte(`{"entities":[
		{"name":"lowImpact","kind":"func","dependents_count":1},
		{"name":"highImpact","kind":"func","dependents_count":42}]}`)}

	sec, err := newSurfaceExtractor().Extract(context.Background(), surfaceInput(g))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(sec.Items) != 2 {
		t.Fatalf("want 2 items, got %d", len(sec.Items))
	}
	if !strings.HasPrefix(sec.Items[0].Text, "highImpact") {
		t.Errorf("highest dependents must sort first, got %q", sec.Items[0].Text)
	}
	if !strings.Contains(sec.Note, "heuristic") {
		t.Errorf("dependents_count caveat must be disclosed, note=%q", sec.Note)
	}
}

func TestSurfaceSuppressesDegenerateEntities(t *testing.T) {
	t.Parallel()

	g := fakeGraph{out: []byte(`{"entities":[
		{"name":"real","dependents_count":5},
		{"name":"bogus","dependents_count":9,"degenerate":true,"degenerate_reason":"no_dependents"}]}`)}

	sec, err := newSurfaceExtractor().Extract(context.Background(), surfaceInput(g))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, item := range sec.Items {
		if strings.Contains(item.Text, "bogus") {
			t.Error("degenerate entity must be suppressed, not dressed up")
		}
	}
	if !strings.Contains(sec.Note, "degenerate") {
		t.Errorf("suppression must be reported, note=%q", sec.Note)
	}
}

func TestSurfaceFallsBackToTouchedFiles(t *testing.T) {
	t.Parallel()

	g := fakeGraph{out: []byte(`{"entities":[]}`)}
	sec, err := newSurfaceExtractor().Extract(context.Background(), surfaceInput(g, "b.go", testFileA))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(sec.Items) != 2 {
		t.Fatalf("want the touched-file fallback, got %d items (note=%q)", len(sec.Items), sec.Note)
	}
	if sec.Items[0].Text != testFileA {
		t.Errorf("fallback should be sorted, got %q first", sec.Items[0].Text)
	}
	if !strings.Contains(sec.Note, "falling back") {
		t.Errorf("fallback must be disclosed, note=%q", sec.Note)
	}
}

func TestSurfaceGraphFailureIsANoteNotAnError(t *testing.T) {
	t.Parallel()

	g := fakeGraph{err: errors.New("graph binary missing")}
	sec, err := newSurfaceExtractor().Extract(context.Background(), surfaceInput(g))
	if err != nil {
		t.Fatalf("a failing graph must not fail the packet: %v", err)
	}
	if len(sec.Items) != 0 {
		t.Errorf("want no items, got %d", len(sec.Items))
	}
	if !strings.Contains(sec.Note, "graph unavailable") {
		t.Errorf("note = %q", sec.Note)
	}
}

func TestSurfaceMalformedJSONIsANote(t *testing.T) {
	t.Parallel()

	g := fakeGraph{out: []byte("not json at all")}
	sec, err := newSurfaceExtractor().Extract(context.Background(), surfaceInput(g))
	if err != nil {
		t.Fatalf("malformed graph output must not fail the packet: %v", err)
	}
	if !strings.Contains(sec.Note, "not valid json") {
		t.Errorf("note = %q", sec.Note)
	}
}

func TestSurfaceAcceptsChangesKey(t *testing.T) {
	t.Parallel()

	g := fakeGraph{out: []byte(`{"changes":[{"name":"fromChanges","dependents_count":3}]}`)}
	sec, err := newSurfaceExtractor().Extract(context.Background(), surfaceInput(g))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(sec.Items) != 1 || !strings.HasPrefix(sec.Items[0].Text, "fromChanges") {
		t.Fatalf("changes[] key not honoured: %+v", sec.Items)
	}
}

func TestSurfaceItemsAreCited(t *testing.T) {
	t.Parallel()

	g := fakeGraph{out: []byte(`{"entities":[{"name":"x","dependents_count":2}]}`)}
	sec, err := newSurfaceExtractor().Extract(context.Background(), surfaceInput(g))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for i, item := range sec.Items {
		if len(item.Cites) == 0 || item.Cites[0].CheckpointID != testCheckpointID {
			t.Errorf("item %d not cited: %+v", i, item.Cites)
		}
	}
}
