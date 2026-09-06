package handoff

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const (
	maxDeadEnds      = 12
	maxOutputChars   = 200
	maxFollowupChars = 240
)

type deadEnd struct {
	Key      string
	Count    int
	Line     int
	Output   string
	Followup string
	Cite     Citation
}

type deadEndsExtractor struct{}

func newDeadEndsExtractor() Extractor { return deadEndsExtractor{} }

func (deadEndsExtractor) Name() string { return SectionDeadEnds }

func (deadEndsExtractor) Extract(_ context.Context, in Input) (Section, error) {
	if len(in.Checkpoints) == 0 {
		return section(SectionDeadEnds, nil, noCheckpointsNote), nil
	}

	byKey := make(map[string]*deadEnd)
	var order []string
	scanned := 0

	for _, cp := range in.Checkpoints {
		if len(cp.Transcript) == 0 {
			continue
		}
		start := 0
		if cp.HasCompactStart {
			start = cp.CompactStart
		}
		for _, c := range scanToolCalls(cp.Transcript, start) {
			scanned++
			key := groupKey(c)
			d, ok := byKey[key]
			if !ok {
				d = &deadEnd{
					Key:      key,
					Line:     c.Line,
					Output:   truncate(cleanWhitespace(c.Output), maxOutputChars),
					Followup: truncate(cleanWhitespace(c.Followup), maxFollowupChars),
					Cite: Citation{
						CheckpointID: cp.ID,
						SessionIndex: cp.SessionIndex,
						Line:         c.Line,
					},
				}
				byKey[key] = d
				order = append(order, key)
			}
			d.Count++
			if d.Followup == "" && c.Followup != "" {
				d.Followup = truncate(cleanWhitespace(c.Followup), maxFollowupChars)
			}
		}
	}

	if len(byKey) == 0 {
		if scanned == 0 && !anyTranscript(in) {
			return section(SectionDeadEnds, nil, "no transcripts available in range"), nil
		}
		return section(SectionDeadEnds, nil, "no failed tool calls found in range"), nil
	}

	ends := make([]*deadEnd, 0, len(byKey))
	for _, k := range order {
		ends = append(ends, byKey[k])
	}
	sort.SliceStable(ends, func(i, j int) bool {
		if ends[i].Count != ends[j].Count {
			return ends[i].Count > ends[j].Count
		}
		return ends[i].Line < ends[j].Line
	})

	var note string
	if len(ends) > maxDeadEnds {
		note = fmt.Sprintf("showing the %d most repeated of %d dead ends", maxDeadEnds, len(ends))
		ends = ends[:maxDeadEnds]
	}

	items := make([]Item, 0, len(ends))
	for _, d := range ends {
		items = append(items, Item{Text: d.describe(), Cites: []Citation{d.Cite}})
	}
	return Section{Name: SectionDeadEnds, Items: items, Note: note}, nil
}

func (d *deadEnd) describe() string {
	var b strings.Builder
	b.WriteString(d.Key)
	if d.Count > 1 {
		fmt.Fprintf(&b, " — failed %d times", d.Count)
	} else {
		b.WriteString(" — failed")
	}
	if d.Output != "" {
		fmt.Fprintf(&b, ": %s", d.Output)
	}
	if d.Followup != "" {
		fmt.Fprintf(&b, " (then: %s)", d.Followup)
	}
	return b.String()
}

func anyTranscript(in Input) bool {
	for _, cp := range in.Checkpoints {
		if len(cp.Transcript) > 0 {
			return true
		}
	}
	return false
}

func cleanWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := s[:limit]
	if i := strings.LastIndexByte(cut, ' '); i > limit/2 {
		cut = cut[:i]
	}
	return cut + "…"
}
