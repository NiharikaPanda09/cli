package handoff

import (
	"context"
	"fmt"
	"time"
)

// Now is swappable so tests get a deterministic GeneratedAt.
var Now = time.Now

// Build runs every extractor and assembles the packet.
//
// One extractor failing never fails the packet: its section is rendered with an
// "(unavailable: ...)" note and the rest still print. An agent that gets four
// good sections is oriented; one that gets an error message is not.
func Build(ctx context.Context, in Input, noGraph bool) Packet {
	return BuildWith(ctx, in, BuildOptions{NoGraph: noGraph})
}

func BuildWith(ctx context.Context, in Input, opts BuildOptions) Packet {
	p := Packet{
		Repo:        in.Repo,
		GeneratedAt: Now().UTC(),
		Head:        in.Head,
	}
	// Privacy Boundary: computed once so every section in this packet carries
	// the same honest signal about the range it was built from. See
	// Input.hasGaps and Section.Incomplete.
	gaps := in.hasGaps()
	for _, x := range allExtractors(opts) {
		sec, err := x.Extract(ctx, in)
		if err != nil {
			sec = Section{Name: x.Name(), Note: fmt.Sprintf("unavailable: %v", err)}
		}
		if sec.Name == "" {
			sec.Name = x.Name()
		}
		if gaps {
			sec.Incomplete = true
		}
		p.Sections = append(p.Sections, sec)
	}
	return p
}
