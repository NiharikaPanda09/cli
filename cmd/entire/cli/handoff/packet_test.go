package handoff

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	apicheckpoint "github.com/entireio/cli/api/checkpoint"
)

func testInput() Input {
	md := &apicheckpoint.Metadata{
		FilesTouched: []string{"cmd/entire/cli/recap.go"},
	}
	return Input{
		Repo: "github.com/acme/cli",
		Head: "01JQ8Z9K2M3N4P5Q6R7S8T9V0W",
		Checkpoints: []Checkpoint{
			{
				ID:           "01JQ8Z9K2M3N4P5Q6R7S8T9V0W",
				SessionIndex: 0,
				CreatedAt:    time.Unix(1_757_000_000, 0),
				Metadata:     md,
				FilesTouched: []string{"cmd/entire/cli/recap.go"},
				Summary: &apicheckpoint.Summary{
					Intent:    "Add a --json flag to entire recap",
					Outcome:   "Flag added; tests still failing",
					OpenItems: []string{"recap_test.go still skips the empty-store case"},
				},
			},
			{
				ID:           "01AAAA9K2M3N4P5Q6R7S8T9V0W",
				SessionIndex: 0,
				CreatedAt:    time.Unix(1_756_000_000, 0),
				Metadata:     &apicheckpoint.Metadata{},
				Summary: &apicheckpoint.Summary{
					Intent:    "add a  --JSON flag to Entire recap", // near-duplicate
					OpenItems: []string{"document the packet shape"},
				},
			},
		},
	}
}

// TestEveryItemIsCited enforces the product's central claim mechanically across
// every registered extractor. If this fails, the packet is making assertions it
// cannot attribute, which is the one thing it must never do.
func TestEveryItemIsCited(t *testing.T) {
	t.Parallel()

	p := Build(context.Background(), testInput(), false)
	if len(p.Sections) == 0 {
		t.Fatal("packet has no sections")
	}
	for _, sec := range p.Sections {
		for i, item := range sec.Items {
			if len(item.Cites) == 0 {
				t.Errorf("section %q item %d (%q) has no citation", sec.Name, i, item.Text)
				continue
			}
			for _, c := range item.Cites {
				if c.CheckpointID == "" {
					t.Errorf("section %q item %d has an empty checkpoint ID", sec.Name, i)
				}
			}
		}
	}
}

// TestNilSummaryDoesNotPanic covers the most likely live failure: Summary is a
// pointer and is nil whenever no summary was generated, which is common for
// exactly the interrupted sessions a handoff is most needed for.
func TestNilSummaryDoesNotPanic(t *testing.T) {
	t.Parallel()

	in := Input{
		Repo: "r",
		Checkpoints: []Checkpoint{
			{ID: "01AAA", Metadata: &apicheckpoint.Metadata{}, Summary: nil},
			{ID: "01BBB", Metadata: &apicheckpoint.Metadata{}, Summary: nil},
		},
	}
	p := Build(context.Background(), in, false)

	for _, sec := range p.Sections {
		if sec.Name != SectionIntent && sec.Name != SectionOpenItems {
			continue
		}
		if len(sec.Items) != 0 {
			t.Errorf("section %q: want no items, got %d", sec.Name, len(sec.Items))
		}
		if sec.Note == "" {
			t.Errorf("section %q: empty section must carry a Note explaining why", sec.Name)
		}
	}

	// It must still render in both formats.
	if err := RenderMarkdown(&bytes.Buffer{}, p); err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if err := RenderJSON(&bytes.Buffer{}, p); err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
}

func TestEmptyInputStillProducesAPacket(t *testing.T) {
	t.Parallel()

	p := Build(context.Background(), Input{Repo: "r"}, false)
	if len(p.Sections) != 5 {
		t.Fatalf("want 5 sections, got %d", len(p.Sections))
	}
	for _, sec := range p.Sections {
		if sec.Note == "" {
			t.Errorf("section %q: empty section must carry a Note", sec.Name)
		}
	}
}

// TestEmptyRangeBlamesCheckpointsNotSummaries pins the note wording apart:
// with nothing to read, "no session summaries" names a symptom and sends the
// reader hunting a summary-generation problem they do not have.
func TestEmptyRangeBlamesCheckpointsNotSummaries(t *testing.T) {
	t.Parallel()

	empty := Build(context.Background(), Input{Repo: "r"}, true)
	for _, sec := range empty.Sections {
		if sec.Name != SectionIntent && sec.Name != SectionOpenItems {
			continue
		}
		if sec.Note != noCheckpointsNote {
			t.Errorf("section %q with no checkpoints: Note = %q, want %q",
				sec.Name, sec.Note, noCheckpointsNote)
		}
	}

	// With checkpoints present but no summaries, the summary note is correct.
	withCPs := Build(context.Background(), Input{
		Repo:        "r",
		Checkpoints: []Checkpoint{{ID: "01AAA", Metadata: &apicheckpoint.Metadata{}}},
	}, true)
	for _, sec := range withCPs.Sections {
		if sec.Name != SectionIntent {
			continue
		}
		if sec.Note != noSummariesNote {
			t.Errorf("section %q with summary-less checkpoints: Note = %q, want %q",
				sec.Name, sec.Note, noSummariesNote)
		}
	}
}

func TestNoGraphOmitsSurfaceSection(t *testing.T) {
	t.Parallel()

	p := Build(context.Background(), testInput(), true)
	for _, sec := range p.Sections {
		if sec.Name == SectionSurface {
			t.Fatal("--no-graph must omit the surface section")
		}
	}
}

func TestIntentDedupesNearDuplicates(t *testing.T) {
	t.Parallel()

	sec, err := newIntentExtractor().Extract(context.Background(), testInput())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(sec.Items) != 1 {
		t.Fatalf("want 1 deduped intent, got %d: %+v", len(sec.Items), sec.Items)
	}
}

// TestOpenItemsMarksRatherThanDeletes pins the deliberate choice that a
// resolved-looking item is annotated, never dropped. Silently removing a
// promise is the failure this product claims to fix.
func TestOpenItemsMarksRatherThanDeletes(t *testing.T) {
	t.Parallel()

	in := Input{
		Checkpoints: []Checkpoint{
			{ // newer: touches the file the older item names
				ID:           "01NEW",
				Metadata:     &apicheckpoint.Metadata{},
				FilesTouched: []string{"cmd/entire/cli/recap_test.go"},
				Summary:      &apicheckpoint.Summary{},
			},
			{
				ID:       "01OLD",
				Metadata: &apicheckpoint.Metadata{},
				Summary: &apicheckpoint.Summary{
					OpenItems: []string{"recap_test.go still skips the empty-store case"},
				},
			},
		},
	}
	sec, err := newOpenItemsExtractor().Extract(context.Background(), in)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(sec.Items) != 1 {
		t.Fatalf("item must be kept, not deleted; got %d items", len(sec.Items))
	}
	if !strings.Contains(sec.Items[0].Text, "(likely closed)") {
		t.Errorf("want a (likely closed) annotation, got %q", sec.Items[0].Text)
	}
}

// TestRenderJSONNeverEmitsNullArrays guards the published encoding: a null
// where a consumer expects an array is how a downstream parser dies on the one
// packet that happened to have an empty section.
func TestRenderJSONNeverEmitsNullArrays(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := RenderJSON(&buf, Build(context.Background(), Input{Repo: "r"}, false)); err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte(": null")) {
		t.Errorf("packet contains a null array:\n%s", buf.String())
	}

	// And it must round-trip.
	var got Packet
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("round-trip: %v", err)
	}
	if got.Repo != "r" {
		t.Errorf("Repo = %q, want %q", got.Repo, "r")
	}
}

func TestUnreadableCheckpointsAreReportedNotSilent(t *testing.T) {
	t.Parallel()

	in := Input{Repo: "r", Listed: 100, Unreadable: 100}
	p := Build(context.Background(), in, true)

	for _, sec := range p.Sections {
		if sec.Note == noCheckpointsNote {
			t.Errorf("section %q claims no checkpoints when 100 were listed but unreadable", sec.Name)
		}
		if !strings.Contains(sec.Note, "100") {
			t.Errorf("section %q should name the count, got %q", sec.Name, sec.Note)
		}
	}
}

func TestTrulyEmptyRangeStillSaysNoCheckpoints(t *testing.T) {
	t.Parallel()

	p := Build(context.Background(), Input{Repo: "r"}, true)
	for _, sec := range p.Sections {
		if sec.Note != noCheckpointsNote {
			t.Errorf("section %q: Note = %q, want %q", sec.Name, sec.Note, noCheckpointsNote)
		}
	}
}
