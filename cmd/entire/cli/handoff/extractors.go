package handoff

// allExtractors is the wiring list: it names every extractor exactly once, in
// packet order.
//
// This file is written once and then never edited again. Each workstream
// implements its own constructor in its own file and deletes the matching stub
// in stubs.go, so no shared file is ever changed twice and the wiring cannot
// conflict during integration. Adding a section -- including one a curveball
// demands -- is a new file plus one stub deletion.
func allExtractors(noGraph bool) []Extractor {
	xs := []Extractor{
		newIntentExtractor(),
		newOpenItemsExtractor(),
		newDeadEndsExtractor(),
	}
	if !noGraph {
		xs = append(xs, newSurfaceExtractor())
	}
	return append(xs, newStoppedExtractor())
}
