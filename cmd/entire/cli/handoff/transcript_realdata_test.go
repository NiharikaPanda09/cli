package handoff

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apicheckpoint "github.com/entireio/cli/api/checkpoint"
)

const compactFixtureRel = "../transcript/compact/testdata/claude_expected2.jsonl"

func realCompactOutput(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(compactFixtureRel))
	if err != nil {
		t.Skipf("compact fixture unavailable: %v", err)
	}
	return data
}

func TestScanToolCallsAgainstRealCompactOutput(t *testing.T) {
	t.Parallel()

	calls := scanToolCalls(realCompactOutput(t), 0)
	if len(calls) == 0 {
		t.Fatal("parser found no errored tool calls in real compact output; the writer format and this parser have diverged")
	}
	for _, c := range calls {
		if c.Status != statusError {
			t.Errorf("non-error call leaked: %+v", c)
		}
		if c.Name == "" {
			t.Error("tool name not parsed from real output")
		}
		if c.Line < 0 {
			t.Errorf("negative line offset %d", c.Line)
		}
	}
}

func TestGroupKeyResolvesRealBashCommands(t *testing.T) {
	t.Parallel()

	calls := scanToolCalls(realCompactOutput(t), 0)
	var named int
	for _, c := range calls {
		key := groupKey(c)
		if key == "" || key == "unknown tool" {
			t.Errorf("group key not resolved for %+v", c)
			continue
		}
		if c.Name == toolBash && key == toolBash {
			t.Error("Bash call fell back to tool name instead of its command")
			continue
		}
		named++
	}
	if named == 0 {
		t.Fatal("no group keys resolved from real output")
	}
}

func TestDeadEndsEndToEndOnRealCompactOutput(t *testing.T) {
	t.Parallel()

	in := Input{
		Repo: "real",
		Checkpoints: []Checkpoint{{
			ID:         "01REAL",
			Metadata:   &apicheckpoint.Metadata{},
			Transcript: realCompactOutput(t),
		}},
	}
	sec, err := newDeadEndsExtractor().Extract(context.Background(), in)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(sec.Items) == 0 {
		t.Fatalf("no dead ends mined from real output; note=%q", sec.Note)
	}
	for i, item := range sec.Items {
		if len(item.Cites) == 0 {
			t.Fatalf("item %d uncited", i)
		}
		if item.Cites[0].CheckpointID != "01REAL" {
			t.Errorf("item %d cites %q", i, item.Cites[0].CheckpointID)
		}
		if strings.Contains(item.Text, "\n") {
			t.Errorf("item %d contains a raw newline, which breaks markdown list rendering", i)
		}
		if len(item.Text) > maxOutputChars+maxFollowupChars+400 {
			t.Errorf("item %d not truncated: %d chars", i, len(item.Text))
		}
	}
}

func TestRealCompactOutputExcludesSuccesses(t *testing.T) {
	t.Parallel()

	data := realCompactOutput(t)
	if !strings.Contains(string(data), `"status":"success"`) {
		t.Skip("fixture has no successful calls to exclude")
	}
	for _, c := range scanToolCalls(data, 0) {
		if c.Status == statusSuccess {
			t.Fatal("a successful tool call reached the miner")
		}
	}
}
