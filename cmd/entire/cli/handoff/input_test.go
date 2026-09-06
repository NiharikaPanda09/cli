package handoff

import (
	"context"
	"errors"
	"testing"
	"time"

	apicheckpoint "github.com/entireio/cli/api/checkpoint"
	"github.com/entireio/cli/cmd/entire/cli/checkpoint/id"
)

type fakeStore struct {
	infos    []apicheckpoint.CheckpointInfo
	attempts int
	fail     bool
}

func (f *fakeStore) List(context.Context) ([]apicheckpoint.CheckpointInfo, error) {
	return f.infos, nil
}

func (f *fakeStore) Read(context.Context, id.CheckpointID) (*apicheckpoint.CheckpointSummary, error) {
	return nil, errors.New("not used")
}

func (f *fakeStore) ReadSessionContent(_ context.Context, cpID id.CheckpointID, _ int) (*apicheckpoint.SessionContent, error) {
	f.attempts++
	if f.fail {
		return nil, errors.New("unhydrated remote checkpoint")
	}
	return &apicheckpoint.SessionContent{
		Metadata: apicheckpoint.Metadata{CheckpointID: cpID},
	}, nil
}

func manyInfos(n int) []apicheckpoint.CheckpointInfo {
	infos := make([]apicheckpoint.CheckpointInfo, n)
	for i := range infos {
		infos[i] = apicheckpoint.CheckpointInfo{
			CheckpointID: id.CheckpointID("01CP"),
			CreatedAt:    time.Unix(int64(1_700_000_000+n-i), 0),
		}
	}
	return infos
}

func TestLoadBoundsAttemptsWhenNothingIsReadable(t *testing.T) {
	t.Parallel()

	store := &fakeStore{infos: manyInfos(500), fail: true}
	in, err := Load(context.Background(), store, LoadOptions{Repo: "r", Limit: 5})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(in.Checkpoints) != 0 {
		t.Fatalf("nothing should load, got %d", len(in.Checkpoints))
	}
	if store.attempts >= 500 {
		t.Errorf("Load walked the whole store (%d attempts); each is a network fetch on a real repo", store.attempts)
	}
	if store.attempts > minLoadAttempts {
		t.Errorf("attempts = %d, want at most %d", store.attempts, minLoadAttempts)
	}
	if !in.Truncated {
		t.Error("giving up early must be reported, not silent")
	}
	if in.Unreadable == 0 {
		t.Error("unreadable checkpoints must be counted")
	}
}

func TestLoadStopsAtLimitWhenReadable(t *testing.T) {
	t.Parallel()

	store := &fakeStore{infos: manyInfos(100)}
	in, err := Load(context.Background(), store, LoadOptions{Repo: "r", Limit: 3})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(in.Checkpoints) != 3 {
		t.Fatalf("want 3 checkpoints, got %d", len(in.Checkpoints))
	}
	if store.attempts != 3 {
		t.Errorf("attempts = %d, want exactly 3 — no wasted fetches", store.attempts)
	}
	if in.Truncated {
		t.Error("a successful bounded read must not report truncation")
	}
}

func TestLoadHonoursCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store := &fakeStore{infos: manyInfos(100)}
	in, err := Load(ctx, store, LoadOptions{Repo: "r", Limit: 10})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if store.attempts != 0 {
		t.Errorf("a cancelled context must stop before any fetch, got %d", store.attempts)
	}
	if !in.Truncated {
		t.Error("cancellation must be reported")
	}
}

func TestLoadOrdersNewestFirst(t *testing.T) {
	t.Parallel()

	store := &fakeStore{infos: manyInfos(5)}
	in, err := Load(context.Background(), store, LoadOptions{Repo: "r", Limit: 5})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for i := 1; i < len(in.Checkpoints); i++ {
		if in.Checkpoints[i].CreatedAt.After(in.Checkpoints[i-1].CreatedAt) {
			t.Fatalf("checkpoint %d is newer than %d; the contract is newest first", i, i-1)
		}
	}
}
