package handoff

import (
	"context"
	"fmt"
	"time"

	apicheckpoint "github.com/entireio/cli/api/checkpoint"
)

const (
	SectionIntent    = "intent"
	SectionOpenItems = "open_items"
	SectionDeadEnds  = "dead_ends"
	SectionSurface   = "surface"
	SectionStopped   = "stopped"
)

type Citation struct {
	CheckpointID string `json:"checkpoint_id"`
	SessionIndex int    `json:"session_index"`
	Line         int    `json:"line,omitempty"`
}

type Item struct {
	Text  string     `json:"text"`
	Cites []Citation `json:"cites"`
}

type Section struct {
	Name  string `json:"name"`
	Items []Item `json:"items"`
	Note  string `json:"note,omitempty"`
	// Incomplete is true when this section was built from a checkpoint range
	// that had unreadable, truncated, or redacted/missing data (e.g. a nil
	// Summary or an empty Transcript). It never means the section is wrong --
	// only that it may not be the whole picture, so a reader must not treat it
	// as authoritative on its own.
	Incomplete bool `json:"incomplete,omitempty"`
}

type Packet struct {
	Repo        string    `json:"repo"`
	GeneratedAt time.Time `json:"generated_at"`
	Head        string    `json:"head_checkpoint_id"`
	Sections    []Section `json:"sections"`
}

type Checkpoint struct {
	ID              string
	SessionIndex    int
	CreatedAt       time.Time
	Metadata        *apicheckpoint.Metadata
	Summary         *apicheckpoint.Summary
	FilesTouched    []string
	Transcript      []byte
	CompactStart    int
	HasCompactStart bool
}

type GraphRunner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

type Input struct {
	Repo        string
	Head        string
	Checkpoints []Checkpoint
	Graph       GraphRunner
	Listed      int
	Unreadable  int
	Truncated   bool
}

// hasGaps reports whether this checkpoint range contains evidence of missing
// or redacted data: unreadable/truncated checkpoints at the range level, or a
// loaded checkpoint with no Summary or no Transcript. It is the signal every
// section uses to mark itself Incomplete -- see the Privacy Boundary note on
// Section.Incomplete. Deliberately over-inclusive: a section that might be
// missing something must say so, even if that particular section did not end
// up needing the missing field.
func (in Input) hasGaps() bool {
	if in.Unreadable > 0 || in.Truncated {
		return true
	}
	for _, cp := range in.Checkpoints {
		if cp.Summary == nil || len(cp.Transcript) == 0 {
			return true
		}
	}
	return false
}

func (in Input) emptyRangeNote() string {
	if in.Unreadable > 0 {
		note := fmt.Sprintf("%d of %d checkpoints in range could not be read; run `entire checkpoint list` to check they are fetched", in.Unreadable, in.Listed)
		if in.Truncated {
			note += "; gave up early to avoid a long wait"
		}
		return note
	}
	if in.Truncated {
		return "gave up reading checkpoints early to avoid a long wait"
	}
	return noCheckpointsNote
}

type Extractor interface {
	Name() string
	Extract(ctx context.Context, in Input) (Section, error)
}

const noCheckpointsNote = "no checkpoints in range"

func cite(cp Checkpoint) []Citation {
	return []Citation{{CheckpointID: cp.ID, SessionIndex: cp.SessionIndex}}
}

func section(name string, items []Item, emptyNote string) Section {
	if len(items) == 0 {
		return Section{Name: name, Items: nil, Note: emptyNote}
	}
	return Section{Name: name, Items: items}
}

func emptyNoteFor(in Input, reason string) string {
	if len(in.Checkpoints) == 0 {
		return in.emptyRangeNote()
	}
	return reason
}
