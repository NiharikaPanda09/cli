package handoff

import (
	"context"
	"strings"
)

// noSummariesNote is shared by every extractor that reads Summary, so the
// degradation reads identically wherever it surfaces.
const noSummariesNote = "no session summaries in range — run with more checkpoints"

type intentExtractor struct{}

func newIntentExtractor() Extractor { return intentExtractor{} }

func (intentExtractor) Name() string { return SectionIntent }

// Extract lists what each session set out to do, newest first.
//
// Near-duplicates are dropped because consecutive checkpoints in one sitting
// routinely share an intent, and repeating it three times pushes the older,
// more informative entries out of the reader's view.
func (intentExtractor) Extract(_ context.Context, in Input) (Section, error) {
	var items []Item
	seen := make(map[string]bool)

	for _, cp := range in.Checkpoints {
		// Summary is a pointer and is nil whenever no summary was generated.
		if cp.Summary == nil {
			continue
		}
		text := strings.TrimSpace(cp.Summary.Intent)
		if text == "" {
			continue
		}
		key := normalizeForDedupe(text)
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, Item{Text: text, Cites: cite(cp)})
	}

	return section(SectionIntent, items, emptyNoteFor(in, noSummariesNote)), nil
}

// normalizeForDedupe collapses case and internal whitespace so that two
// summaries differing only in formatting compare equal. Deliberately not
// cleverer than that: stemming or fuzzy matching would silently merge two
// genuinely different intents, and losing one is worse than showing both.
func normalizeForDedupe(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
