package handoff

import (
	"fmt"
	"io"
	"strings"
)

// sectionTitles are the human headings. The machine-readable names in the JSON
// packet stay stable regardless of what we call them here.
var sectionTitles = map[string]string{
	SectionIntent:    "What we were trying to do",
	SectionOpenItems: "Still open",
	SectionDeadEnds:  "Already tried — don't repeat",
	SectionSurface:   "Blast radius",
	SectionStopped:   "Where it stopped",
}

// RenderMarkdown writes the packet as markdown.
//
// No TTY dependence: no pager, no colour, no width probing. The output is
// identical piped and interactive, because the primary reader is an agent that
// has no terminal.
func RenderMarkdown(w io.Writer, p Packet) error {
	b := &strings.Builder{}

	fmt.Fprintf(b, "# Handoff — %s\n\n", p.Repo)
	fmt.Fprintf(b, "Generated %s", p.GeneratedAt.Format("2006-01-02 15:04 MST"))
	if p.Head != "" {
		fmt.Fprintf(b, " · HEAD checkpoint `%s`", p.Head)
	}
	b.WriteString("\n")

	if packetHasIncompleteSection(p) {
		b.WriteString("\n> **⚠ Incomplete context.** Some checkpoints in range were " +
			"unreadable, missing, or had redacted fields. Sections marked " +
			"**(incomplete)** below may not be the whole picture — verify before " +
			"treating them as authoritative.\n")
	}

	for _, sec := range p.Sections {
		title := sectionTitles[sec.Name]
		if title == "" {
			title = sec.Name
		}
		if sec.Incomplete {
			title += " (incomplete)"
		}
		fmt.Fprintf(b, "\n## %s\n\n", title)

		if len(sec.Items) == 0 {
			note := sec.Note
			if note == "" {
				note = "nothing recorded"
			}
			fmt.Fprintf(b, "_%s_\n", note)
			continue
		}
		for _, item := range sec.Items {
			fmt.Fprintf(b, "- %s %s\n", item.Text, formatCites(item.Cites))
		}
		if sec.Note != "" {
			fmt.Fprintf(b, "\n_%s_\n", sec.Note)
		}
	}

	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("write handoff markdown: %w", err)
	}
	return nil
}

// packetHasIncompleteSection reports whether any section in the packet was
// built from a gap-containing range. See Section.Incomplete.
func packetHasIncompleteSection(p Packet) bool {
	for _, sec := range p.Sections {
		if sec.Incomplete {
			return true
		}
	}
	return false
}

// formatCites renders the provenance suffix. Every item has at least one
// citation by contract, so this is never empty in practice -- but it degrades
// quietly rather than panicking if that ever stops being true.
func formatCites(cites []Citation) string {
	if len(cites) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cites))
	for _, c := range cites {
		s := shortID(c.CheckpointID)
		if c.Line > 0 {
			s = fmt.Sprintf("%s:%d", s, c.Line)
		}
		parts = append(parts, s)
	}
	return "`[" + strings.Join(parts, " ") + "]`"
}

// shortID trims a checkpoint ID for display. The full ID stays in the JSON
// packet, which is what any tool follows up with.
func shortID(id string) string {
	const n = 8
	if len(id) <= n {
		return id
	}
	return id[:n]
}
