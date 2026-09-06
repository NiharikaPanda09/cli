package handoff

import (
	"context"
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
		return noCheckpointsNote
	}
	return reason
}
