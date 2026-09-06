package handoff

// allExtractors is the wiring list: it names every extractor exactly once, in
// packet order.
//
// This file is written once and then never edited again. Each workstream
// implements its own constructor in its own file and deletes the matching stub
// in stubs.go, so no shared file is ever changed twice and the wiring cannot
// conflict during integration. Adding a section -- including one a curveball
// demands -- is a new file plus one stub deletion.
type BuildOptions struct {
	NoGraph  bool
	Searcher VectorSearcher
	Ask      string
}

func allExtractors(opts BuildOptions) []Extractor {
	xs := []Extractor{
		newIntentExtractor(),
		newOpenItemsExtractor(),
		newDeadEndsExtractor(),
	}
	if !noGraph(opts) {
		xs = append(xs, newSurfaceExtractor())
	}
	if opts.Searcher != nil || opts.Ask != "" {
		xs = append(xs, newGlobalMemoryExtractor(opts.Searcher, opts.Ask))
	}
	return append(xs, newStoppedExtractor())
}

func noGraph(o BuildOptions) bool { return o.NoGraph }
