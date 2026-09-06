package handoff

import "context"

// Placeholder extractors for sections still in flight on another branch.
//
// Each returns an empty Section carrying a "not implemented" Note, so the
// command runs end-to-end from the first commit and every section is visibly
// accounted for rather than silently missing.
//
// WHEN YOU LAND YOUR REAL EXTRACTOR, DELETE ITS LINE BELOW. That deletion is
// the only edit you make outside your own files.
//
//	newDeadEndsExtractor -> Workstream B (deadends.go)
//	newSurfaceExtractor  -> Workstream B (surface.go), optional
type notImplemented struct{ name string }

func (n notImplemented) Name() string { return n.name }

func (n notImplemented) Extract(context.Context, Input) (Section, error) {
	return Section{Name: n.name, Note: "not implemented"}, nil
}

func newDeadEndsExtractor() Extractor { return notImplemented{SectionDeadEnds} }
func newSurfaceExtractor() Extractor  { return notImplemented{SectionSurface} }
