package handoff

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apicheckpoint "github.com/entireio/cli/api/checkpoint"
)

const (
	testCheckpointID = "01CP"
	toolBash         = "Bash"
)

func loadFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "transcript.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func TestScanToolCallsOnlyReturnsErrors(t *testing.T) {
	t.Parallel()

	calls := scanToolCalls(loadFixture(t), 0)
	if len(calls) != 4 {
		t.Fatalf("want 4 errored calls, got %d", len(calls))
	}
	for _, c := range calls {
		if c.Status != statusError {
			t.Errorf("call %q: status = %q, want %q", c.Name, c.Status, statusError)
		}
		if strings.Contains(c.Output, "ok") && c.Name == toolBash && c.Input["command"] == "go build ./..." {
			t.Error("successful call leaked into results")
		}
	}
}

func TestScanToolCallsSkipsUnparseableLines(t *testing.T) {
	t.Parallel()

	if calls := scanToolCalls(loadFixture(t), 0); len(calls) == 0 {
		t.Fatal("an unparseable line must be skipped, not abort the scan")
	}
}

func TestScanToolCallsKeepsFollowupText(t *testing.T) {
	t.Parallel()

	calls := scanToolCalls(loadFixture(t), 0)
	if len(calls) == 0 {
		t.Fatal("no calls")
	}
	if !strings.Contains(calls[0].Followup, "does not exist yet") {
		t.Errorf("followup text dropped, got %q", calls[0].Followup)
	}
}

func TestScanToolCallsHonoursCompactStart(t *testing.T) {
	t.Parallel()

	all := scanToolCalls(loadFixture(t), 0)
	sliced := scanToolCalls(loadFixture(t), 3)
	if len(sliced) >= len(all) {
		t.Fatalf("a non-zero start must drop earlier lines: all=%d sliced=%d", len(all), len(sliced))
	}
	for _, c := range sliced {
		if c.Line < 3 {
			t.Errorf("line %d predates the slice start", c.Line)
		}
	}
}

func TestScanToolCallsDedupesRepeatedHeadLine(t *testing.T) {
	t.Parallel()

	line := `{"v":1,"type":"assistant","id":"msg_dup","content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"x"},"result":{"output":"boom","status":"error"}}]}`
	doubled := []byte(line + "\n" + line + "\n")

	if calls := scanToolCalls(doubled, 0); len(calls) != 1 {
		t.Fatalf("a repeated head line must dedupe to 1 call, got %d", len(calls))
	}
}

func TestGroupKeyPrefersCommandThenPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call toolCall
		want string
	}{
		{"bash command", toolCall{Name: toolBash, Input: map[string]any{"command": "go test"}}, "go test"},
		{"file path", toolCall{Name: "Edit", Input: map[string]any{"file_path": "a.go"}}, "a.go"},
		{"tool name fallback", toolCall{Name: "WebFetch"}, "WebFetch"},
		{"unknown", toolCall{}, "unknown tool"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := groupKey(tt.call); got != tt.want {
				t.Errorf("groupKey = %q, want %q", got, tt.want)
			}
		})
	}
}

func fixtureInput(t *testing.T, start int) Input {
	t.Helper()
	return Input{
		Repo: "r",
		Checkpoints: []Checkpoint{{
			ID:              testCheckpointID,
			Metadata:        &apicheckpoint.Metadata{},
			Transcript:      loadFixture(t),
			CompactStart:    start,
			HasCompactStart: false,
		}},
	}
}

func TestDeadEndsGroupsRepeatedCommand(t *testing.T) {
	t.Parallel()

	sec, err := newDeadEndsExtractor().Extract(context.Background(), fixtureInput(t, 0))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(sec.Items) == 0 {
		t.Fatal("no dead ends found")
	}
	top := sec.Items[0].Text
	if !strings.Contains(top, "go test ./... -run TestRecap") {
		t.Errorf("most repeated dead end should sort first, got %q", top)
	}
	if !strings.Contains(top, "failed 2 times") {
		t.Errorf("want a count of 2, got %q", top)
	}
}

func TestDeadEndsLegacyCheckpointReadsFromLineZero(t *testing.T) {
	t.Parallel()

	legacy, err := newDeadEndsExtractor().Extract(context.Background(), fixtureInput(t, 99))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(legacy.Items) == 0 {
		t.Fatal("HasCompactStart=false must ignore CompactStart and read from line 0")
	}
}

func TestDeadEndsAreCitedWithALine(t *testing.T) {
	t.Parallel()

	sec, err := newDeadEndsExtractor().Extract(context.Background(), fixtureInput(t, 0))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for i, item := range sec.Items {
		if len(item.Cites) == 0 {
			t.Fatalf("item %d has no citation", i)
		}
		if item.Cites[0].CheckpointID != testCheckpointID {
			t.Errorf("item %d cites %q", i, item.Cites[0].CheckpointID)
		}
	}
}

func TestDeadEndsTruncatesOutput(t *testing.T) {
	t.Parallel()

	sec, err := newDeadEndsExtractor().Extract(context.Background(), fixtureInput(t, 0))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	var found bool
	for _, item := range sec.Items {
		if strings.Contains(item.Text, "AKIAIOSFODNN7EXAMPLE") {
			found = true
			if len(item.Text) > maxOutputChars+maxFollowupChars+200 {
				t.Errorf("output not truncated, len=%d", len(item.Text))
			}
			if !strings.Contains(item.Text, "…") {
				t.Error("truncated output must be marked with an ellipsis")
			}
		}
	}
	if !found {
		t.Fatal("expected the long-output dead end in the section")
	}
}

func TestDeadEndsNoTranscriptExplainsWhy(t *testing.T) {
	t.Parallel()

	in := Input{Checkpoints: []Checkpoint{{ID: testCheckpointID, Metadata: &apicheckpoint.Metadata{}}}}
	sec, err := newDeadEndsExtractor().Extract(context.Background(), in)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(sec.Items) != 0 {
		t.Fatalf("want no items, got %d", len(sec.Items))
	}
	if !strings.Contains(sec.Note, "no transcripts available") {
		t.Errorf("Note = %q", sec.Note)
	}
}

func TestDeadEndsNoErrorsExplainsWhy(t *testing.T) {
	t.Parallel()

	clean := []byte(`{"v":1,"type":"assistant","id":"m","content":[{"type":"tool_use","id":"t","name":"Bash","input":{"command":"go build"},"result":{"output":"ok","status":"success"}}]}` + "\n")
	in := Input{Checkpoints: []Checkpoint{{ID: testCheckpointID, Metadata: &apicheckpoint.Metadata{}, Transcript: clean}}}

	sec, err := newDeadEndsExtractor().Extract(context.Background(), in)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(sec.Items) != 0 {
		t.Fatalf("a successful call must not appear, got %d items", len(sec.Items))
	}
	if !strings.Contains(sec.Note, "no failed tool calls") {
		t.Errorf("Note = %q", sec.Note)
	}
}
