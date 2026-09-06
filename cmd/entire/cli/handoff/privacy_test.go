package handoff

import (
	"context"
	"strings"
	"testing"

	apicheckpoint "github.com/entireio/cli/api/checkpoint"
)

// redactedCheckpointFixture models a checkpoint from a sensitive repository:
// Metadata is present (so files-touched and attribution still resolve), but
// Summary and Transcript -- the two fields most likely to carry prompt-shaped
// content -- are exactly what a security-conscious redaction pass would strip
// or simply never have generated. No separate fixture file was supplied with
// the Noon Curveball card, so this is constructed directly from its
// description: "sensitive fields are redacted or unavailable."
func redactedCheckpointFixture() Checkpoint {
	return Checkpoint{
		ID:           "01REDACTEDCHECKPOINT0000001",
		SessionIndex: 0,
		Metadata: &apicheckpoint.Metadata{
			FilesTouched: []string{"internal/billing/charge.go"},
		},
		FilesTouched: []string{"internal/billing/charge.go"},
		// Summary: nil -- redacted or never generated.
		// Transcript: nil -- redacted or never generated.
	}
}

// TestPrivacyBoundary_RedactedCheckpointStillProducesAUsablePacket is the
// Noon Curveball's required test: at least one test using redacted or missing
// Checkpoint data. It asserts the three behaviors the card demands together --
// the packet must still be useful, it must say plainly that it is incomplete,
// and it must never present that incomplete data as complete.
func TestPrivacyBoundary_RedactedCheckpointStillProducesAUsablePacket(t *testing.T) {
	t.Parallel()

	in := Input{
		Repo: "gh/acme/sensitive-repo",
		Checkpoints: []Checkpoint{
			redactedCheckpointFixture(),
			{
				ID:           "01NORMALCHECKPOINT00000001",
				SessionIndex: 0,
				Metadata:     &apicheckpoint.Metadata{},
				FilesTouched: []string{"internal/billing/refund.go"},
				Summary:      &apicheckpoint.Summary{Intent: "Add refund support", Outcome: "Done"},
				Transcript:   []byte(`{"v":1,"type":"assistant"}`),
			},
		},
	}

	p := Build(context.Background(), in, true)

	// 1. Existing local functionality must continue to work: the packet still
	// builds, still cites everything it does say, and the one checkpoint that
	// DOES have a summary still surfaces it.
	foundIntent := false
	for _, sec := range p.Sections {
		for i, item := range sec.Items {
			if len(item.Cites) == 0 || item.Cites[0].CheckpointID == "" {
				t.Errorf("section %q item %d is uncited: %q", sec.Name, i, item.Text)
			}
			if sec.Name == SectionIntent {
				foundIntent = true
			}
		}
	}
	if !foundIntent {
		t.Error("the non-redacted checkpoint's intent should still be usable output")
	}

	// 2. The interface must clearly distinguish complete from incomplete
	// context, and must never quietly present the gap as a complete result.
	if !packetHasIncompleteSection(p) {
		t.Fatal("a packet built over a redacted checkpoint must mark at least one section Incomplete")
	}

	var md strings.Builder
	if err := RenderMarkdown(&md, p); err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if !strings.Contains(md.String(), "Incomplete context") {
		t.Error("markdown output must visibly warn about incomplete context, not just carry a hidden flag")
	}
}

// TestPrivacyBoundary_CompleteRangeIsNeverFlaggedIncomplete guards the other
// direction of the same claim: a range with nothing missing must not be
// mislabelled, which would train a reader to ignore the warning.
func TestPrivacyBoundary_CompleteRangeIsNeverFlaggedIncomplete(t *testing.T) {
	t.Parallel()

	p := Build(context.Background(), testInput(), true)
	if packetHasIncompleteSection(p) {
		t.Error("a fully-populated checkpoint range must not be marked incomplete")
	}
}

// TestPrivacyBoundary_AskQueryIsRedactedBeforeLeavingTheProcess is the other
// half of the Curveball: raw prompts must not reach the new external service.
// --ask is typed verbatim by the user, so it is exactly the "raw prompt" the
// security team is worried about.
func TestPrivacyBoundary_AskQueryIsRedactedBeforeLeavingTheProcess(t *testing.T) {
	t.Parallel()

	const secret = "AKIAABCDEFGHIJKLMNOP" // recognizable AWS access-key-id shape
	s := &fakeSearcher{}
	if _, err := newGlobalMemoryExtractor(s, "why did "+secret+" get committed").
		Extract(context.Background(), intentInput()); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if strings.Contains(s.gotText, secret) {
		t.Errorf("raw secret reached the outgoing query text: %q", s.gotText)
	}
}

// TestPrivacyBoundary_AutoDerivedQueryIsRedactedToo covers the other query
// source: when --ask is empty, the query falls back to Summary.Intent, which
// is equally prompt-shaped and must go through the same redaction.
func TestPrivacyBoundary_AutoDerivedQueryIsRedactedToo(t *testing.T) {
	t.Parallel()

	const secret = "AKIAABCDEFGHIJKLMNOP"
	in := Input{
		Checkpoints: []Checkpoint{{
			ID:       testCheckpointID,
			Metadata: &apicheckpoint.Metadata{},
			Summary:  &apicheckpoint.Summary{Intent: "rotate " + secret + " immediately"},
		}},
	}
	s := &fakeSearcher{}
	if _, err := newGlobalMemoryExtractor(s, "").Extract(context.Background(), in); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if strings.Contains(s.gotText, secret) {
		t.Errorf("raw secret from Summary.Intent reached the outgoing query text: %q", s.gotText)
	}
}
