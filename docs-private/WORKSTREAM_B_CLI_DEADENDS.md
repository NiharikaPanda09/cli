# Workstream B — the dead-end miner

**You own this end-to-end. Nobody else will touch these files.**
Read §1–§3 before writing code; they are the whole context you need. You do **not** need to read
`IMPLEMENTATION_ROADMAP.md` or `IMPLEMENTATION_PLAN.md` — everything relevant is reproduced here.

---

## §1 — What we are building (context)

We are adding a command to the Entire CLI:

```
entire handoff            # human-readable markdown
entire handoff --json     # machine-readable "handoff packet"
```

**The problem.** When an AI coding session ends, the next session starts blind. It re-tries approaches
that already failed, re-asks answered questions, and silently drops promises. Today the only fix is a
human pasting context by hand.

**The product.** `entire handoff` reads the repo's Entire checkpoints and emits a *verifiable* packet
in five sections — **intent, open items, dead ends, surface (blast radius), where it stopped**. The
central claim is that **every line carries a checkpoint citation**.

### Why your workstream is the one that matters

Entire already ships an official `session-handoff` skill that summarises sessions. The obvious
criticism of this whole project is **"isn't this just a wrapper around that?"**

**Your miner is the answer.** Sections 1, 2 and 5 read summaries that already exist. Yours reads
transcript internals that **no CLI command and no exported helper exposes today** — the
`result.status` of every tool call — and mines the failures. "Three sessions ago someone tried
`go test -run TestRecap` and it failed with `undefined: recapFlags`" is not something the existing
skill can say. That is the differentiator, and it is why you get a dedicated person.

---

## §2 — Do this FIRST: the go/no-go (10 minutes, before any code)

The entire feature rests on one assumption: **Claude Code writes `"status":"error"` blocks into
`transcript.jsonl`.** Verify it before investing an hour.

```bash
entire enable -y --agent claude-code

# Make one throwaway commit whose session contains a DELIBERATELY FAILING tool call,
# e.g. ask the agent to run:  cat /nope
git log -1 --format=%B                     # must show an "Entire-Checkpoint:" trailer
entire checkpoint list

git cat-file -p refs/entire/checkpoints/<shard>/<id>    # must contain 0/transcript.jsonl
grep -c '"status":"error"' <that transcript>
```

**Report that number out loud immediately.**
If it is `0`, the feature changes shape — say so at once rather than debugging quietly. The fallback
is to mine `Summary.Friction` text instead, which is a much weaker product, and the integrator needs
to know within minutes, not at noon.

---

## §3 — Rules that will fail CI

Extracted from the repo's `CLAUDE.md`. (`BUILD_RULES.md` is referenced by our planning docs but
**does not exist** — don't go looking for it.)

1. **Standard library only. Do NOT edit `go.mod`/`go.sum`.** A `go.sum` conflict during late
   integration is unrecoverable at this clock.
2. **`t.Parallel()` in every test function and subtest.** `t.Setenv()`/`t.Chdir()` **panic** after
   `t.Parallel()` — but your tests are pure functions over fixture bytes, so this should never come up.
3. **Never touch the real repo CWD in tests.** Use `testutil.InitRepo(t, tmpDir)` if you need a repo
   at all — you probably don't.
4. **`dupl` blocks CI at threshold 75** (~20 duplicated lines). If two functions end up the same
   shape, factor the shared part into a helper. Check with `mise run dup`.
5. **`mise run fmt` then `mise run lint`** — in that order; `fmt` rewrites files.
6. **Never run `mise run test:e2e`** — real, paid API calls.
7. Iterate with **`go test ./cmd/entire/cli/handoff/...`**. **Do not run `mise run test:ci` while
   iterating** — minutes long, and three people running it concurrently makes every laptop unusable.
   Your tests are pure functions over fixture bytes; they run in under a second. **Keep them that way**
   — that is what lets you run them every ten minutes.
8. **Log only operational metadata** — `{checkpoint_id, session_index, tool_calls_scanned,
   errors_found, duration_ms}`. **Never** log prompts, tool inputs, or error output. That content goes
   to stdout because the user asked for it; it must never reach `.entire/logs/`.
9. **No TTY dependence** — no pager, no picker, no `huh`.

---

## §4 — The contract (FROZEN — code against this now)

The integrator is writing `cmd/entire/cli/handoff/packet.go` with exactly this. **Start writing your
files immediately; they will not compile until that lands (~11:05), and that is expected.** You are
writing pure functions and can reason about them on paper.

```go
package handoff

type Citation struct {
    CheckpointID string `json:"checkpoint_id"`
    SessionIndex int    `json:"session_index"`
    Line         int    `json:"line,omitempty"` // transcript.jsonl offset; dead ends only
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

// Checkpoint is one checkpoint's already-loaded data. Ordered NEWEST FIRST.
type Checkpoint struct {
    ID           string
    SessionIndex int
    CreatedAt    time.Time
    Metadata     *apicheckpoint.Metadata // never nil
    Summary      *apicheckpoint.Summary  // MAY BE NIL
    FilesTouched []string
    Transcript   []byte // transcript.jsonl bytes; nil when unavailable
    CompactStart    int
    HasCompactStart bool
}

type Input struct {
    Repo        string
    Head        string
    Checkpoints []Checkpoint // newest first
    Graph       GraphRunner  // nil when --no-graph
}

type Extractor interface {
    Name() string
    Extract(ctx context.Context, in Input) (Section, error)
}
```

**Two rules every extractor obeys:**

1. **Every `Item` carries at least one `Citation`.** This is the product's central claim. The
   integrator owns a test that enforces it mechanically across all extractors. An uncited item is a bug.
2. **Failure is a `Note`, not an `error`.** If you cannot do your job, return a `Section` with empty
   `Items`, a populated `Note`, and **`nil` error**. Reserve the error return for genuine bugs.
   Partial packets are the product — a packet that refuses to print because one section failed is
   worse than useless at noon.

**Your entry point** is `newDeadEndsExtractor()`. The integrator has already written the wiring line
referencing it, plus a temporary stub. **When you land the real one, delete the stub line in
`stubs.go` — that one line is the only edit you make outside your own files.**

---

## §5 — Your files

| File | What |
| --- | --- |
| `cmd/entire/cli/handoff/transcript.go` | the v1-line parser |
| `cmd/entire/cli/handoff/deadends.go` | grouping + the `Extractor` |
| `cmd/entire/cli/handoff/deadends_test.go` | table tests |
| `cmd/entire/cli/handoff/testdata/transcript.jsonl` | hand-written fixture |

### 5.1 — `transcript.go`: write your own parser

> **Do NOT use `compact.BuildCondensedEntries`. Verified, do not re-litigate:**
> `cmd/entire/cli/transcript/compact/parse.go:88-105` builds each `tool` entry from `block["name"]`
> and `block["input"]` **only** — it never reads `block["result"]`, so `result.status` is discarded
> before it reaches any caller. `CondensedEntry` is `{Type, Content, ToolName, ToolDetail}`
> (`parse.go:10-15`): no status field, and no line index either, which you need for slicing.
>
> The status **is** on disk: `compact.go:476` writes `blocks[idx]["result"]`, and `buildToolResult`
> sets `Status: "error"` when the call failed, else `"success"`.
>
> **Do not try to export `toolResultJSON`** — it is unexported, the `compact` package is shared by
> seven agent sniffers, and changing it drags upstream tests with it.

The line shape you are parsing:

```json
{"v":1,"agent":"claude-code","cli_version":"0.42.0","type":"assistant","ts":"…","id":"msg_x",
 "content":[{"type":"text","text":"…"},
            {"type":"tool_use","id":"…","name":"Bash","input":{…},
             "result":{"output":"…","status":"error"}}]}
```

```go
type toolCall struct {
    Line   int    // offset in transcript.jsonl — becomes Citation.Line
    Name   string
    Input  map[string]any
    Status string // "success" | "error" | ""
    Output string
}

func scanToolCalls(transcript []byte, from int) ([]toolCall, []string, error)
```

- Split on `\n`; `json.Unmarshal` each line into `{Type string; Content json.RawMessage}`.
- **Skip blank and unparseable lines — never fail the command on one bad line.**
- For `type == "assistant"`, unmarshal `Content` into `[]map[string]json.RawMessage`; for blocks with
  `type == "tool_use"`, read `name`, `input`, `result.status`, `result.output`.
- **Keep the assistant `text` block that follows an errored call.** That text is *why the approach was
  abandoned*, and it is the highest-value string in the entire packet. Do not drop it.

### 5.2 — Slicing: the one trap

`CompactStart` / `HasCompactStart` are handed to you on the `Checkpoint`; the integrator has already
called `GetCompactTranscriptStart()`. **Do not call it yourself.**

- `HasCompactStart == false` → **legacy checkpoint, start at line 0.** It means "legacy", *not* "error".
- **Never use `GetTranscriptStart()`** — it indexes raw `full.jsonl`, a **different coordinate
  system**. Using it against `transcript.jsonl` produces silently wrong line numbers, which means
  silently wrong citations, which is the one thing this product cannot ship.
- **Tolerate one repeated line at the head.** The boundary rounds toward inclusion when a streaming
  message straddles it, so the slice may repeat up to one compact line. Dedupe by
  `(line.id, block index)`.

### 5.3 — `deadends.go`: grouping

- Group key: `input.command` for Bash, `input.file_path` for edits, else the tool name.
- One entry per key: **count**, **first line offset**, error output **truncated to ~200 chars**, and
  the following assistant text.
- Order by count descending; **cap at 12 entries**.
- Every emitted `Item` gets a `Citation{CheckpointID, SessionIndex, Line}` — `Line` is what makes a
  dead end verifiable, and yours is the only section that populates it.

> **Truncation is a security requirement, not cosmetics.** A transcript can contain a pasted token or
> key in tool output. Truncate to ~200 chars, and never log the output at all (§3.8).

### 5.4 — Fixture and tests

`testdata/transcript.jsonl` must contain, exercised as a table test:

- a legacy checkpoint (nil `compact_transcript_start`) → read from line 0
- a **repeated head line** → deduped
- **two errors on the same Bash command** → grouped into one entry with count 2
- one error on a **file path** → grouped by `file_path`
- one **success** → must NOT appear in the output
- one **unparseable line** → skipped, command still succeeds

```bash
go test ./cmd/entire/cli/handoff/...
```

---

## §6 — Scope disclosure you must write yourself

**Claude Code only.** `compact.Compact` sniffs OpenCode, Gemini, pi, Codex, Copilot and Droid before
the Claude/Cursor path, and amp/goose/kilo/kiro/qwen produce no usable `transcript.jsonl` at all.

**Write this sentence into `BUILDATHON.md` yourself** — one line, plainly. Do not leave it for the
integrator at 14:40. Disclosing your own limits is explicitly scored, and a judge finds this in ninety
seconds anyway.

## §7 — If you finish early, in this order

1. **`surface.go`** — section 4, graph blast radius. Note `entire graph impact` **requires
   `--symbol`**; it will not take a bare file path. Prefer `entire graph diff` across the range, which
   returns entity-level changes with `dependents_count` and needs no symbol. **Suppress** entries
   flagged `degenerate` rather than dressing them up. Shell out behind the `GraphRunner` interface so
   tests inject a fake.
2. **MCP tool** — `mcp.go:185 mcpToolDefs()` add `entire_handoff`; `mcp.go:208 handleMCPToolCall` add
   the case. Note the hardcoded `Arguments struct{ Command string }` at `mcp.go:211` needs widening.
   **Skill first, MCP second** — that ordering is a positioning argument, so do not invert it.

## §8 — Branch and hand-off

- Branch **`ws/deadends`**. Push there; **never push to `main`** — the integrator merges `--no-ff`.
- `git fetch && git rebase origin/main` before asking for a merge. Should be a clean no-op. **If it
  is not, you have touched someone else's file — stop and say so.**
- Commit small and often with real messages. Checkpoint quality is scored on preserving intent and
  decision context; a `wip` message wastes a scored artefact.

## §9 — Definition of done

- [ ] Go/no-go number reported out loud (§2)
- [ ] `go test ./cmd/entire/cli/handoff/...` green, every test `t.Parallel()`
- [ ] All six fixture cases covered (§5.4)
- [ ] Every emitted `Item` carries a `Citation` with a correct `Line`
- [ ] Error output truncated to ~200 chars; nothing sensitive logged
- [ ] Stub line deleted from `stubs.go`; no other file outside your four touched
- [ ] `mise run fmt && mise run lint` clean; `mise run dup` shows no new duplication
- [ ] Claude-Code-only disclosure written into `BUILDATHON.md`
