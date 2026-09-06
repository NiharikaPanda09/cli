package handoff

import (
	"context"
	"fmt"
	"sort"
	"time"

	apicheckpoint "github.com/entireio/cli/api/checkpoint"
	"github.com/entireio/cli/cmd/entire/cli/checkpoint/id"
)

// Store is the slice of the checkpoint store this package needs. Narrowing it
// to three methods keeps the extractors testable with a small fake instead of a
// real git repository.
type Store interface {
	List(ctx context.Context) ([]apicheckpoint.CheckpointInfo, error)
	Read(ctx context.Context, checkpointID id.CheckpointID) (*apicheckpoint.CheckpointSummary, error)
	ReadSessionContent(ctx context.Context, checkpointID id.CheckpointID, sessionIndex int) (*apicheckpoint.SessionContent, error)
}

// LoadOptions bounds the checkpoint walk.
type LoadOptions struct {
	Repo       string
	Head       string
	Limit      int    // max checkpoints; <= 0 means DefaultLimit
	Checkpoint string // start here instead of the newest
	Session    string // only checkpoints from this session id
	Graph      GraphRunner
}

// DefaultLimit is how many checkpoints back we read when unbounded. Ten covers
// a working session without making the packet too long for an agent to act on.
const DefaultLimit = 10

const (
	LoadBudget       = 10 * time.Second
	minLoadAttempts  = 20
	loadAttemptScale = 3
)

// Load walks the store newest-first and assembles the Input.
//
// Per-checkpoint failures are skipped rather than fatal: a single unreadable
// checkpoint -- unpushed, pruned, or written by a newer CLI -- must not deny
// the caller the other nine.
func Load(ctx context.Context, store Store, opts LoadOptions) (Input, error) {
	in := Input{Repo: opts.Repo, Head: opts.Head, Graph: opts.Graph}

	infos, err := store.List(ctx)
	if err != nil {
		return in, fmt.Errorf("list checkpoints: %w", err)
	}
	// List order is not guaranteed; the whole contract is newest-first.
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].CreatedAt.After(infos[j].CreatedAt)
	})

	if opts.Checkpoint != "" {
		infos = skipUntil(infos, opts.Checkpoint)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}

	deadline := Now().Add(LoadBudget)
	maxAttempts := limit * loadAttemptScale
	if maxAttempts < minLoadAttempts {
		maxAttempts = minLoadAttempts
	}
	attempts := 0

	for _, info := range infos {
		if len(in.Checkpoints) >= limit {
			break
		}
		if ctx.Err() != nil || attempts >= maxAttempts || Now().After(deadline) {
			in.Truncated = true
			break
		}
		if opts.Session != "" && !checkpointHasSession(info, opts.Session) {
			continue
		}
		attempts++
		in.Listed++
		if info.ListedStub {
			in.Unreadable++
			// Names-only remote-discovery entry with nothing hydrated behind it.
			continue
		}
		cp, ok := loadCheckpoint(ctx, store, info)
		if !ok {
			in.Unreadable++
			continue
		}
		in.Checkpoints = append(in.Checkpoints, cp)
	}
	return in, nil
}

// loadCheckpoint reads one checkpoint's session data. Reports false when there
// is nothing usable to contribute.
func loadCheckpoint(ctx context.Context, store Store, info apicheckpoint.CheckpointInfo) (Checkpoint, bool) {
	// Session 0 is the primary session; a checkpoint always has at least one.
	content, err := store.ReadSessionContent(ctx, info.CheckpointID, 0)
	if err != nil || content == nil {
		return Checkpoint{}, false
	}
	md := content.Metadata

	cp := Checkpoint{
		ID:           info.CheckpointID.String(),
		SessionIndex: 0,
		CreatedAt:    info.CreatedAt,
		Metadata:     &md,
		Summary:      md.Summary,
		FilesTouched: info.FilesTouched,
		Transcript:   content.Transcript,
	}
	if len(cp.FilesTouched) == 0 {
		cp.FilesTouched = md.FilesTouched
	}

	// Resolve the compact-transcript offset exactly once here so no extractor
	// calls it twice and none reaches for GetTranscriptStart(), which indexes a
	// different file in a different coordinate system.
	cp.CompactStart, cp.HasCompactStart = md.GetCompactTranscriptStart()

	return cp, true
}

// skipUntil drops entries newer than the requested checkpoint, so --checkpoint
// reads "start the handoff here". An unknown ID yields the full list rather
// than an empty one: showing everything is a recoverable surprise, showing
// nothing looks like data loss.
func skipUntil(infos []apicheckpoint.CheckpointInfo, want string) []apicheckpoint.CheckpointInfo {
	for i, info := range infos {
		if info.CheckpointID.String() == want {
			return infos[i:]
		}
	}
	return infos
}

func checkpointHasSession(info apicheckpoint.CheckpointInfo, want string) bool {
	if info.SessionID == want {
		return true
	}
	for _, id := range info.SessionIDs {
		if id == want {
			return true
		}
	}
	return false
}
