package handoff

import (
	"context"
	"path"
	"strings"
)

type openItemsExtractor struct{}

func newOpenItemsExtractor() Extractor { return openItemsExtractor{} }

func (openItemsExtractor) Name() string { return SectionOpenItems }

// Extract unions the open items across the range and marks the ones that a
// later checkpoint probably resolved.
//
// "Probably" is doing real work here, and the design turns on it: an item is
// annotated `(likely closed)` rather than deleted. A dropped promise is the
// exact failure this product exists to fix, so a heuristic is never allowed to
// remove a line -- only to add a hint. The reader keeps the final say.
//
// Checkpoints arrive newest first, so "a later checkpoint" means one earlier in
// the slice.
func (openItemsExtractor) Extract(_ context.Context, in Input) (Section, error) {
	var items []Item
	seen := make(map[string]bool)

	for i, cp := range in.Checkpoints {
		if cp.Summary == nil {
			continue
		}
		// Files touched by any strictly-later checkpoint.
		later := filesTouchedBefore(in.Checkpoints, i)

		for _, raw := range cp.Summary.OpenItems {
			text := strings.TrimSpace(raw)
			if text == "" {
				continue
			}
			key := normalizeForDedupe(text)
			if seen[key] {
				continue
			}
			seen[key] = true

			if mentionsAny(text, later) {
				text += " (likely closed)"
			}
			items = append(items, Item{Text: text, Cites: cite(cp)})
		}
	}

	return section(SectionOpenItems, items, emptyNoteFor(in, noSummariesNote)), nil
}

// filesTouchedBefore collects files touched by checkpoints newer than index i.
// The slice is newest-first, so those are the entries at [0, i).
func filesTouchedBefore(cps []Checkpoint, i int) map[string]bool {
	files := make(map[string]bool)
	for _, cp := range cps[:i] {
		for _, f := range cp.FilesTouched {
			files[f] = true
			// Match on the base name too: an open item usually says
			// "recap_test.go", not the full repo-relative path.
			files[path.Base(f)] = true
		}
	}
	return files
}

// mentionsAny reports whether the item text names any of the given paths.
func mentionsAny(text string, files map[string]bool) bool {
	lower := strings.ToLower(text)
	for f := range files {
		if f == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(f)) {
			return true
		}
	}
	return false
}
