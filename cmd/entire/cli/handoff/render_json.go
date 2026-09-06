package handoff

import (
	"encoding/json"
	"fmt"
	"io"
)

// RenderJSON writes the machine-readable packet.
//
// This encoding is a published interface: it is what a fresh agent reads to
// bootstrap and what the Databricks ingest lane consumes on stdin. Adding a
// field is safe; renaming or removing one is a breaking change for both.
//
// Sections is normalised to a non-nil slice and every Section's Items to a
// non-nil array, so consumers can iterate without a null check -- a null where
// an array was expected is the classic way a downstream parser dies on the one
// packet that happened to have an empty section.
func RenderJSON(w io.Writer, p Packet) error {
	if p.Sections == nil {
		p.Sections = []Section{}
	}
	for i := range p.Sections {
		if p.Sections[i].Items == nil {
			p.Sections[i].Items = []Item{}
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(p); err != nil {
		return fmt.Errorf("encode packet: %w", err)
	}
	return nil
}
