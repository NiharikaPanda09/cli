package handoff

import (
	"context"
	"fmt"
	"strings"
)

type stoppedExtractor struct{}

func newStoppedExtractor() Extractor { return stoppedExtractor{} }

func (stoppedExtractor) Name() string { return SectionStopped }

// Extract describes where the most recent session left off.
//
// Unlike the other sections this one reads the newest checkpoint only: it is a
// position report, not a history. It deliberately draws on metadata rather than
// the summary, so it still says something useful when no summary was generated
// -- which is the common case for a session that was interrupted, and exactly
// when a handoff matters most.
func (stoppedExtractor) Extract(_ context.Context, in Input) (Section, error) {
	if len(in.Checkpoints) == 0 {
		return section(SectionStopped, nil, noCheckpointsNote), nil
	}
	cp := in.Checkpoints[0]
	cites := cite(cp)

	var items []Item
	add := func(format string, args ...any) {
		items = append(items, Item{Text: fmt.Sprintf(format, args...), Cites: cites})
	}

	if cp.Summary != nil {
		if outcome := strings.TrimSpace(cp.Summary.Outcome); outcome != "" {
			add("%s", outcome)
		}
	}

	if in.Head != "" {
		add("HEAD is at checkpoint %s", in.Head)
	}

	if md := cp.Metadata; md != nil {
		if len(md.FilesTouched) > 0 {
			add("Last touched: %s", strings.Join(limit(md.FilesTouched, 8), ", "))
		}
		if a := md.Attribution; a != nil && a.TotalLinesChanged > 0 {
			add("Attribution: %.0f%% agent across %d changed lines",
				a.AgentPercentage, a.TotalLinesChanged)
		}
		if m := md.SessionMetrics; m != nil && m.TurnCount > 0 {
			add("Session ran %d turns", m.TurnCount)
		}
		if t := md.TokenUsage; t != nil && t.OutputTokens > 0 {
			add("Tokens: %d in / %d out", t.InputTokens, t.OutputTokens)
		}
	}

	return section(SectionStopped, items, "no position data on the latest checkpoint"), nil
}

// limit caps a slice for display, appending an ellipsis entry when it truncates
// so the reader can tell the list is partial rather than complete.
func limit(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	out := make([]string, 0, n+1)
	out = append(out, xs[:n]...)
	return append(out, fmt.Sprintf("… +%d more", len(xs)-n))
}
