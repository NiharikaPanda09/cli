package handoff

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const (
	maxSurfaceEntries = 12
	graphTimeout      = 20 * time.Second
)

type graphEntity struct {
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	File             string `json:"file"`
	Path             string `json:"path"`
	Change           string `json:"change"`
	DependentsCount  int    `json:"dependents_count"`
	Degenerate       bool   `json:"degenerate"`
	DegenerateReason string `json:"degenerate_reason"`
}

type graphDiff struct {
	Entities []graphEntity `json:"entities"`
	Changes  []graphEntity `json:"changes"`
}

func (d graphDiff) entities() []graphEntity {
	if len(d.Entities) > 0 {
		return d.Entities
	}
	return d.Changes
}

type execGraphRunner struct{}

func (execGraphRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, graphTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "entire", append([]string{"graph"}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run entire graph: %w", err)
	}
	return out, nil
}

type surfaceExtractor struct{}

func newSurfaceExtractor() Extractor { return surfaceExtractor{} }

func (surfaceExtractor) Name() string { return SectionSurface }

func (surfaceExtractor) Extract(ctx context.Context, in Input) (Section, error) {
	if len(in.Checkpoints) == 0 {
		return section(SectionSurface, nil, in.emptyRangeNote()), nil
	}

	runner := in.Graph
	if runner == nil {
		runner = execGraphRunner{}
	}

	out, err := runner.Run(ctx, "diff", "--repo", ".", "--format", "json")
	if err != nil {
		return section(SectionSurface, nil, fmt.Sprintf("graph unavailable: %v", err)), nil
	}

	var diff graphDiff
	if err := json.Unmarshal(out, &diff); err != nil {
		//nolint:nilerr // a degraded section is a Note, never a failed packet
		return section(SectionSurface, nil, "graph output was not valid json"), nil
	}

	entities := diff.entities()
	if len(entities) == 0 {
		return fallbackSurface(in, "graph returned no entities"), nil
	}

	var kept []graphEntity
	degenerate := 0
	for _, e := range entities {
		if e.Degenerate {
			degenerate++
			continue
		}
		if e.DependentsCount <= 0 {
			continue
		}
		kept = append(kept, e)
	}
	if len(kept) == 0 {
		reason := "graph returned no non-degenerate entities"
		if degenerate > 0 {
			reason = fmt.Sprintf("all %d graph entities were degenerate", degenerate)
		}
		return fallbackSurface(in, reason), nil
	}

	sort.SliceStable(kept, func(i, j int) bool {
		return kept[i].DependentsCount > kept[j].DependentsCount
	})

	var note string
	if len(kept) > maxSurfaceEntries {
		note = fmt.Sprintf("showing the %d highest-impact of %d entities", maxSurfaceEntries, len(kept))
		kept = kept[:maxSurfaceEntries]
	}
	if degenerate > 0 {
		note = appendNote(note, fmt.Sprintf("%d degenerate entities suppressed", degenerate))
	}
	note = appendNote(note, "dependents_count is a heuristic textual index and can undercount")

	cites := cite(in.Checkpoints[0])
	items := make([]Item, 0, len(kept))
	for _, e := range kept {
		items = append(items, Item{Text: describeEntity(e), Cites: cites})
	}
	return Section{Name: SectionSurface, Items: items, Note: note}, nil
}

func describeEntity(e graphEntity) string {
	name := firstNonEmpty(e.Name, e.File, e.Path, "unknown entity")
	var b strings.Builder
	b.WriteString(name)
	if e.Kind != "" {
		fmt.Fprintf(&b, " (%s)", e.Kind)
	}
	if e.Change != "" {
		fmt.Fprintf(&b, " %s", e.Change)
	}
	fmt.Fprintf(&b, " — %d dependents", e.DependentsCount)
	return b.String()
}

func fallbackSurface(in Input, reason string) Section {
	files := make(map[string]bool)
	var order []string
	for _, cp := range in.Checkpoints {
		for _, f := range cp.FilesTouched {
			if f == "" || files[f] {
				continue
			}
			files[f] = true
			order = append(order, f)
		}
	}
	if len(order) == 0 {
		return section(SectionSurface, nil, reason)
	}
	sort.Strings(order)
	if len(order) > maxSurfaceEntries {
		order = order[:maxSurfaceEntries]
	}
	cites := cite(in.Checkpoints[0])
	items := make([]Item, 0, len(order))
	for _, f := range order {
		items = append(items, Item{Text: f, Cites: cites})
	}
	return Section{
		Name:  SectionSurface,
		Items: items,
		Note:  appendNote(reason, "falling back to the touched-file list"),
	}
}

func appendNote(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + "; " + add
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
