# Continuity — 3-person parallel implementation roadmap

**Project:** `entire handoff` — a verifiable handoff packet for AI coding sessions (Track E1).
**Source of truth for scope:** `APPLICATION_ANSWER.md`. **For rules:** `BUILD_RULES.md`.
**This document supersedes the schedule in `IMPLEMENTATION_PLAN.md`** (see §0). The plan's
*technical* content is still correct and verified — only its clock is wrong.

---

## §0 — Two corrections that change the whole plan. Read before anything else.

### 0.1 The deadline is 15:00, not 16:00 — and we are 80 minutes behind

`IMPLEMENTATION_PLAN.md:4` says "build window 09:00–16:00 IST … Code freeze 16:00."
The Participant Guide (`BTW Buildathon 2026 - Participant Guide.md`, lines 3 and 14) says:

> "The submission deadline is 3:00 PM IST." — line 3
> "1:00–3:00 Build session and submission … submit before 3:00 PM." — line 14

**The guide wins.** It is the newer file (09:42 vs 09:14) and it is the organiser's document.

This roadmap was written at **10:19 IST**. Actual remaining budget:

| | Plan assumed | Reality |
| --- | --- | --- |
| Start of implementation | 09:15 | **not started — no fork exists** |
| Curveball | 12:00 | 12:00 (unchanged) |
| Freeze | 16:00 | **15:00** |
| Total build minutes | 405 | **~275** |

That is **32% less time than the plan was written for**, and the first 80 minutes are already
spent. Every schedule below is re-baselined. Two consequences you must accept now:

- **The Databricks lane (`IMPLEMENTATION_PLAN.md` Step 10) is cut.** It was scoped as a 13:00–15:30
  stretch inside a 7-hour day. In a 4h40m day with a mandatory curveball it cannot land, and a
  half-built Delta table costs main-challenge points (technical implementation, demo reproducibility)
  without earning Databricks points. Recommend not opting in. *This is a team call — if you overrule
  it, cut Workstream C's graph section (§4.3 C4) instead, not the curveball response.*
- **MCP (`IMPLEMENTATION_PLAN.md` Step 9) is cut** unless the curveball is trivial. Skill first was
  always the positioning argument; MCP was always optional.

### 0.2 The fork does not exist yet

```
cli/  entire-graph/  external-agents/   → all at commit 1bf566c, remote DeshDeepakKant/entireio
```

These are **research clones committed into the planning repo on 3–4 Sep**. They are not the build
fork and nothing written in them counts. Participant Guide line 93: *"All implementation must happen
in the clone created through the Entire mirror workflow."*

**Creating the fork and mirror is the single blocking dependency for all three people.** It is
Milestone M0 and it is owned by A, starting now.

---

## §1 — Team roles

Assign these three names before you read further. Everything below is keyed to them.

| | Owner | Role | Also owns |
| --- | --- | --- | --- |
| **A** | ______ | **Spine** — command, packet contract, renderers, registration | **Integration owner** + **submission owner** |
| **B** | ______ | **Dead-end miner** — the differentiating feature | Transcript fixtures |
| **C** | ______ | **Sections + Skill + Graph** | **Demo owner** + graph evidence |

Participant Guide lines 41–42 require a named **submission owner** and a named **demo owner**, and
they should be different people. A submits (they hold the integration branch); C demos (they own the
skill, which *is* the demo). C must be able to explain B's miner — budget 5 minutes at M4 for B to
walk C through it.

---

## §2 — The dependency graph

Only two things are blocking. Everything else is genuinely parallel.

```
M0  fork + mirror + clone            (A, ~20 min)   ← BLOCKS ALL THREE PEOPLE
     │
     ├──────────────┬──────────────────┐
     ▼              ▼                  ▼
M1  packet.go       B: enable +        C: enable + graph install
    contract (A)       go/no-go grep      + graph evidence #1
     │  ← BLOCKS COMPILATION, NOT WRITING (contract is frozen in §3 below)
     ├──────────────┬──────────────────┐
     ▼              ▼                  ▼
    A: render     B: transcript.go   C: intent/openitems/stopped
       handoff.go     deadends.go       then setup_handoff_skill.go
     │              │                  │
     └──────────────┴──────────────────┘
                    ▼
M2  INTEGRATION GATE 1 — 11:35, stable commit before noon
                    ▼
M3  CURVEBALL 12:00 — fresh session bootstraps via our own skill
                    ▼
M4  INTEGRATION GATE 2 — 14:00
                    ▼
M5  SUBMIT — 14:50
```

**The critical insight that makes this parallel:** the `Input`/`Section` contract is frozen *in this
document* (§3). B and C write their files against §3 immediately, without waiting for A's
`packet.go` to exist. Their code will not compile until A pushes at M1 — that is fine, they are
writing pure functions and can reason about them on paper. **Nobody blocks on anybody.**

---

## §3 — THE CONTRACT (frozen — A writes this verbatim, B and C code against it now)

`cmd/entire/cli/handoff/packet.go`. **A must not change this after 11:00.** If it needs to change,
A announces it out loud and B and C stop typing until they have heard the change.

```go
package handoff

import (
	"context"
	"time"

	apicheckpoint "github.com/entireio/cli/api/checkpoint"
)

// ---------- Output side: what extractors produce ----------

// Citation is the provenance of one Item. Every Item must carry at least one.
type Citation struct {
	CheckpointID string `json:"checkpoint_id"`
	SessionIndex int    `json:"session_index"`
	Line         int    `json:"line,omitempty"` // transcript.jsonl offset; dead ends only
}

type Item struct {
	Text  string     `json:"text"`
	Cites []Citation `json:"cites"`
}

// Section is one of the five parts of the packet. Note carries a degradation
// reason ("no session summaries in range", "graph degenerate: no_dependents").
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

// ---------- Input side: what extractors may read ----------

// Checkpoint is one checkpoint's worth of already-loaded data. Assembled once
// by input.go so no extractor touches the store. Ordered NEWEST FIRST.
type Checkpoint struct {
	ID           string
	SessionIndex int
	CreatedAt    time.Time
	Metadata     *apicheckpoint.Metadata // never nil
	Summary      *apicheckpoint.Summary  // MAY BE NIL — no summary generated
	FilesTouched []string
	Transcript   []byte // transcript.jsonl bytes; nil when unavailable
	// CompactStart is Metadata.GetCompactTranscriptStart(). HasCompactStart
	// false == legacy checkpoint == read from line 0. See §6.2.
	CompactStart    int
	HasCompactStart bool
}

// GraphRunner shells out to `entire graph`. An interface so tests inject a fake
// and --no-graph is a one-line bypass.
type GraphRunner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

type Input struct {
	Repo        string
	Head        string       // HEAD's Entire-Checkpoint trailer, or ""
	Checkpoints []Checkpoint // newest first
	Graph       GraphRunner  // nil when --no-graph
}

type Extractor interface {
	Name() string
	Extract(ctx context.Context, in Input) (Section, error)
}
```

### 3.1 The rule every extractor obeys

**Every `Item` carries at least one `Citation`.** This is the application's central claim ("Every
line carries a checkpoint ID" — `APPLICATION_ANSWER.md:22`). A owns the test that enforces it
mechanically across all five extractors (`packet_test.go`). If your extractor can produce an
uncited item, it is wrong.

### 3.2 Failure is a Note, not an error

An extractor that cannot do its job returns a `Section` with an empty `Items` and a populated
`Note`, and `nil` error. Reserve the error return for genuine bugs. `runHandoff` renders a
failed section as `(unavailable: <err>)` and continues. **Partial packets are the product** — a
packet that refuses to print because the graph is down is worse than useless at 12:00.

### 3.3 Dependencies

**Standard library only.** No new entries in `go.mod`. A `go.sum` conflict at 13:50 is a
merge you cannot afford. If you think you need a dependency, you don't.

---

## §4 — Workstreams

### 4.1 Workstream A — Spine, contract, renderers, integration

**Owns these files. Nobody else edits them.**

| File | What |
| --- | --- |
| `cmd/entire/cli/handoff.go` | cobra command, `handoffFlags`, `runHandoff` |
| `cmd/entire/cli/handoff/packet.go` | **the contract (§3)** |
| `cmd/entire/cli/handoff/input.go` | store open, checkpoint walk, `Input` assembly |
| `cmd/entire/cli/handoff/extractors.go` | the wiring list — **stubbed at M1, never edited again** |
| `cmd/entire/cli/handoff/render_md.go` | markdown renderer (default) |
| `cmd/entire/cli/handoff/render_json.go` | `--json` packet |
| `cmd/entire/cli/handoff/packet_test.go` | citation invariant across all extractors |
| `cmd/entire/cli/root.go` | **one line** at ~line 198 |
| `cmd/entire/cli/agent_help_cmd.go` | **one line** in `agentHelpClassification` |

**A0 (10:20–10:40) — M0, the unblock. Do this before anything else.**

```bash
# 1. Fork entireio/cli on GitHub (NOW — after 09:00, so this is rule-legal)
# 2. Mirror + clone (Participant Guide lines 99–104)
entire login
entire repo mirror create           # select your fork, INDIA region
entire repo clone /gh/DeshDeepakKant/cli
cd cli
# 3. Announce the clone URL to B and C IMMEDIATELY — they are blocked until you do
# 4. Enable (Guide line 108)
entire enable -y --agent claude-code
entire status
# 5. Verify the build works BEFORE writing code (no CGO needed — cli has no tree-sitter)
mise run build
```

Then copy `BUILD_RULES.md` into the fork root and commit it — it is the agent's rulebook.

**A1 (10:40–11:05) — the contract and the skeleton.** In this order:

1. `packet.go` — §3 verbatim. **Push this the moment it compiles.** B and C are waiting.
2. `extractors.go` — the wiring, with all five constructors referenced but only A's own stubs:
   ```go
   func allExtractors(noGraph bool) []Extractor {
       xs := []Extractor{
           newIntentExtractor(),     // C
           newOpenItemsExtractor(),  // C
           newDeadEndsExtractor(),   // B
       }
       if !noGraph {
           xs = append(xs, newSurfaceExtractor()) // C
       }
       return append(xs, newStoppedExtractor())   // C
   }
   ```
   **A also writes a one-line stub for each of B's and C's constructors in a file called
   `stubs.go`, returning an extractor whose `Extract` returns an empty Section with
   `Note: "not implemented"`.** B and C delete their own stub line when they land the real one.
   This is what makes the wiring file never conflict.
3. `handoff.go` — flags (`--session`, `--checkpoint`, `--format md|json`, `--limit 10`,
   `--no-graph`), mirroring `recap.go:40-60`.
4. `input.go` — store open, verbatim from `resume.go:198`:
   ```go
   stores, err := checkpoint.Open(ctx, repo, checkpoint.OpenOptions{
       BlobFetcher: FetchBlobsByHash,
       RefFetcher:  FetchCheckpointRef,
       ReadRemotes: strategy.CheckpointReadRemotes(ctx),
   })
   ```
   `stores.Persistent` gives you `List(ctx)`, `Read(ctx, id)`,
   `ReadSessionMetadataAndPrompts(ctx, id, i)`, `ReadSessionContent(ctx, id, i)`
   (verified at `api/checkpoint/interfaces.go:13-24`). `SessionContent.Transcript` is the bytes.
5. Registration — `root.go`, beside `newRecapCmd()` at line 198:
   ```go
   cmd.AddCommand(inGroup(newHandoffCmd(), groupSessions))
   ```
   and `agent_help_cmd.go` (~line 142, beside `"recap"`):
   ```go
   "handoff": {agentHelpAudienceReadOnly, false},   // unlisted, read-only
   ```
   *Judgment call, per `BUILD_RULES.md` §3:* `read-only` is defensible — handoff writes nothing.
   `listed: false` is the safe default. Say both in `BUILDATHON.md` in one line.

**Exit criterion for A1:** `entire handoff` prints resolved checkpoint IDs and a session count,
with three "not implemented" sections. **Commit and push.** The project is now unblocked.

**A2 (11:05–11:35) — renderers.** `render_md.go` and `render_json.go`. `--json` is the
machine-readable packet `cli#1125` asked for by name — keep the shape stable and document it.
Markdown must not depend on a TTY (`BUILD_RULES.md` §4). No pager, no picker, no `huh`.

**A3 (11:35–11:50) — INTEGRATION GATE 1.** See §7.

**A4 (12:00+) — curveball triage and integration.** A does not implement the curveball alone;
A decides who implements which part of it and holds the merge.

---

### 4.2 Workstream B — The dead-end miner

**This is the product.** It is the one thing `entire handoff` does that the shipped
`session-handoff` skill does not, and it is why this is not a wrapper.

**Owns these files. Nobody else edits them.**

| File | What |
| --- | --- |
| `cmd/entire/cli/handoff/transcript.go` | the v1-line parser |
| `cmd/entire/cli/handoff/deadends.go` | grouping + the `Extractor` |
| `cmd/entire/cli/handoff/deadends_test.go` | table tests |
| `cmd/entire/cli/handoff/testdata/transcript.jsonl` | hand-written fixture |

**B0 (10:20–10:40) — clone as soon as A announces the URL, then run the go/no-go.**

```bash
entire enable -y --agent claude-code
# Make one throwaway commit containing a DELIBERATELY FAILING tool call (e.g. `cat /nope`)
git log -1 --format=%B                      # must show an Entire-Checkpoint: trailer
entire checkpoint list
git cat-file -p refs/entire/checkpoints/<shard>/<id>   # must contain 0/transcript.jsonl
grep -c '"status":"error"' <that transcript>
```

**That last grep is the go/no-go for the whole product.** Report the number to A and C out loud.
If it is 0, say so immediately — §8 cut line 1 fires and B's work changes shape.

**B1 (10:40–11:05) — `transcript.go`.** Write your own parser. Do **not** use
`compact.BuildCondensedEntries`.

> **Verified:** `cmd/entire/cli/transcript/compact/parse.go:88-105` builds each `tool` entry from
> `block["name"]` and `block["input"]` only — it never reads `block["result"]`. `CondensedEntry`
> is `{Type, Content, ToolName, ToolDetail}` (`parse.go:10-15`): no status field, no line index.
> The status **is** on disk — `compact.go:476` writes `blocks[idx]["result"]`, and
> `buildToolResult` (`compact.go:486-496`) sets `Status: "error"` when `tr.isError`, else
> `"success"`. `toolResultJSON` (`compact.go:52-57`) is `{output, status, file, matchCount}` and is
> unexported. Do not try to export it: the package is shared by seven agent sniffers.

The line shape you are parsing (verified, `compact.go:27-37` and the doc comment at `compact.go:80`):

```json
{"v":1,"agent":"claude-code","cli_version":"0.42.0","type":"assistant","ts":"…","id":"msg_x",
 "content":[{"type":"text","text":"…"},
            {"type":"tool_use","id":"…","name":"Bash","input":{…},
             "result":{"output":"…","status":"error"}}]}
```

```go
type toolCall struct {
	Line   int    // offset in transcript.jsonl — this is what Citation.Line carries
	Name   string
	Input  map[string]any
	Status string // "success" | "error" | ""
	Output string
}

func scanToolCalls(transcript []byte, from int) ([]toolCall, []string, error)
```

- Split on `\n`; `json.Unmarshal` each into `{Type string; Content json.RawMessage}`.
- **Skip blank and unparseable lines — never fail the command on one bad line.**
- For `type == "assistant"`, unmarshal `Content` into `[]map[string]json.RawMessage`; for blocks
  with `type == "tool_use"`, read `name`, `input`, `result.status`, `result.output`.
- **Keep the assistant `text` block that follows an errored call.** That text is *why the approach
  was abandoned* and it is the highest-value string in the whole packet.

**Slicing — the one trap.** `Checkpoint.CompactStart` / `HasCompactStart` are given to you in the
`Input`; A has already called `GetCompactTranscriptStart()`. Rules:
- `HasCompactStart == false` → **legacy checkpoint, start at line 0.**
- Never use `GetTranscriptStart()` — that indexes raw `full.jsonl`, a different coordinate system
  (`metadata.go:488-495` vs `:498-507`).
- **Tolerate one repeated line at the head.** `metadata.go:430-441` documents it: the boundary
  "rounds toward inclusion when a streaming message straddles it", so the slice "may repeat up to
  one compact line at its head". Dedupe by `(line.id, block index)`.

**B2 (11:05–11:35) — `deadends.go` + tests.**

- Group by `input.command` for Bash, `input.file_path` for edits, else the tool name.
- One entry per key: count, first line offset, error output **truncated to ~200 chars**, and the
  following assistant text. Order by count desc, cap at 12.
- **Truncation is a security requirement, not cosmetics** (`BUILD_RULES.md` §6): a transcript can
  contain a pasted token.
- **Log only `{checkpoint_id, session_index, tool_calls_scanned, errors_found, duration_ms}`.**
  Never log prompts, tool inputs, or error output — that content goes to stdout, which the user
  asked for, and never to `.entire/logs/`.

**Fixture must contain, as a table test:** a legacy nil `compact_transcript_start`; a repeated head
line; two errors on the same Bash command; one error on a file path; one *success* that must not
appear in the output; one unparseable line that must be skipped.

**Test command — do NOT run `mise run test:ci` while iterating** (see §7.3):
```bash
go test ./cmd/entire/cli/handoff/...
```

**B3 — MVP scope disclosure.** Claude Code only. `compact.Compact` sniffs OpenCode, Gemini, pi,
Codex, Copilot and Droid before the Claude/Cursor path, and `external-agents#67` reports
amp/goose/kilo/kiro/qwen produce no usable `transcript.jsonl` at all. **Write this sentence into
`BUILDATHON.md` yourself** — do not leave it for A at 14:40.

---

### 4.3 Workstream C — Sections, skill, graph

**Owns these files. Nobody else edits them.**

| File | What |
| --- | --- |
| `cmd/entire/cli/handoff/intent.go` | section 1 |
| `cmd/entire/cli/handoff/openitems.go` | section 2 |
| `cmd/entire/cli/handoff/stopped.go` | section 5 |
| `cmd/entire/cli/handoff/surface.go` | section 4 (graph) |
| `cmd/entire/cli/handoff/sections_test.go` | tests for the above |
| `cmd/entire/cli/setup_handoff_skill.go` | the managed skill |
| `cmd/entire/cli/setup_handoff_skill_test.go` | four scaffold cases |
| `cmd/entire/cli/setup.go` | **one line** — `HandoffSkill bool` on `EnableOptions` |

**C0 (10:20–10:45) — clone, enable, graph, and capture required evidence #1.**

```bash
entire enable -y --agent claude-code
entire plugin install graph          # Guide line 121 — do not skip; the plan omitted it
entire graph version
entire graph init-agents --repo .
# Then START A FRESH AGENT SESSION (Guide line 125) so it receives graph instructions
```

**Then immediately capture graph evidence #1** and paste it into a scratch file. The Participant
Guide (lines 125–128) requires **three** graph artefacts and they are worth **15 points**:

1. a graph search or definition lookup — `entire graph def --repo . --symbol runHandoff --format json`
2. a relationship/impact analysis **before** a high-risk change — do this before B's miner lands
3. a final semantic diff of the submitted implementation — at M4

> **This is separate from `surface.go`.** The three artefacts above are evidence about *our own
> development process* and they are mandatory. `surface.go` puts graph *into the product* and is
> optional (cut line 2). **Do not conflate them — a team can ship `surface.go` and still score
> zero on "Use of Entire Graph" by never recording the three artefacts.** Capture them early.

**C1 (10:45–11:10) — sections 1, 2, 5.** All three read `Summary`
(`{Intent, Outcome, Learnings, Friction, OpenItems}`, verified `metadata.go:595-601`). No
transcript parsing.

1. **`intent.go`** — `Summary.Intent` per checkpoint, newest first, near-duplicates dropped
   (normalise case and whitespace; do not get clever).
2. **`openitems.go`** — union of `Summary.OpenItems`, minus any item whose text names a path
   appearing in a **later** checkpoint's `FilesTouched`. **Mark those `likely closed` in a separate
   list rather than deleting them** — a silently dropped promise is the exact failure this product
   claims to fix.
3. **`stopped.go`** — last checkpoint's `Metadata.Attribution` (`initial_attribution`),
   `TokenUsage`, `SessionMetrics`, plus `Input.Head`.

**Guard:** `Summary` is a `*Summary` and **is nil when no summary was generated**. When every
checkpoint in range has a nil Summary, emit
`Note: "no session summaries in range — run with more checkpoints"` and no items. Test this case.

**Watch the duplication linter** (`BUILD_RULES.md` §9): three extractors with the same shape is
exactly what `dupl` at threshold 75 blocks CI on. Put the citation-building and section-assembly
in one helper and let each extractor be only the part that differs. Check with `mise run dup`
before you push.

**C2 (11:10–11:35) — the managed skill. This is the curveball insurance; it must exist by 11:35.**

Ship as a **skill, not MCP**: `cli#292` is closed `wontfix` in favour of skills. Clone
`setup_search_skill.go` exactly — the helpers you need already exist and are already in the
`osroot` allowlist:

```go
repoRoot, err := paths.WorktreeRoot(ctx)          // setup_search_skill.go:53
root, err := openScaffoldRoot(repoRoot)            // setup_managed_scaffold.go:105
result, err := writeManagedScaffold(root, relPath, content, isManagedHandoffSkill)
                                                   // setup_managed_scaffold.go:67
```

- Marker const: `ENTIRE-MANAGED HANDOFF SKILL v1`, checked by `isManagedHandoffSkill`.
- `handoffSkillTemplate(agentName)` returns
  `filepath.Join(".claude", "skills", "entire-handoff", "SKILL.md")` for
  `agent.AgentNameClaudeCode`.
- One line on `EnableOptions` (`setup.go:93`, beside `SearchSkill bool`), and wire
  `setupOptionalHandoffSkill` / `...ForNames` beside the search-skill calls.
- Test: clone the four cases in `setup_search_skill_test.go` — created, unchanged, updated,
  skipped-conflict.

> **Do not deviate from this path.** `writeManagedScaffold` already gives you
> created/updated/unchanged/skipped-conflict semantics *and* is already in `allowedRootBases`.
> Rolling your own means adding a new root base, which means touching
> `osroot/rootbase_guard_test.go`, which fails the build, at 12:30, for nothing
> (`BUILD_RULES.md` §7).

**Skill body — one instruction:** *run `entire handoff --json` first, before reading any file*,
plus the packet's section names. That single sentence is the "fresh agent bootstraps in one call"
claim, made real, and it is what you demo at 12:05.

**C3 (11:35–11:50) — `BUILDATHON.md` skeleton.** Use the outline at Participant Guide lines
244–256 **verbatim as headings**. Fill in problem, track, architecture now while it is quiet.

**C4 (13:00+, optional) — `surface.go`, section 4.** Only if the curveball has not consumed you.

> **Design correction, verified:** `entire graph impact` **requires `--symbol`** —
> `entire-graph/internal/cli/impact.go:288` returns `"impact requires --symbol"`. The plan's
> "for each file, run `entire graph impact`" does not work: you have `FilesTouched`, which are
> paths, not symbols. **Recommended shape:** run `entire graph diff` (alias `analyze`) across the
> range instead — it returns an entity-level change list with `dependents_count` per entity, which
> is exactly "blast radius of what was touched" and needs no symbol input.
> Then read `degenerate` / `degenerate_reason` (`impact.go:103-104`) and **suppress** degenerate
> entries rather than dressing them up.

Shell out via `exec.CommandContext` behind `GraphRunner` so tests inject a fake and `--no-graph`
is a one-line bypass.

**Disclose in `BUILDATHON.md`:** `dependents_count` is a heuristic textual identifier index, not a
graph query (`entire-graph/internal/sem/dependents.go`); it emits `W_ANALYSIS_BUDGET_EXCEEDED` and
undercounts on budget exhaustion (`dependents.go:388`), and `entire-graph/internal/cli/help.go:319`
calls it "a heuristic dependent count" in its own help text. A judge who opens that file finds it
in ninety seconds — so put it in `BUILDATHON.md` first.

---

## §5 — Git branches and merge protocol

```
main                    ← integration branch, in the FORK. This is what you submit.
 ├─ ws/spine            ← A
 ├─ ws/deadends         ← B
 └─ ws/sections         ← C
```

**Rules:**

1. **Nobody pushes to `main` except A.** B and C push their own branch and tell A.
2. **No pull requests.** At this clock, PR review is a luxury. A merges locally:
   `git checkout main && git merge --no-ff ws/deadends`. `--no-ff` keeps the workstream visible in
   the history, which reads well for judges.
3. **Rebase on `main` before you ask for a merge:** `git fetch && git rebase origin/main`. You
   own only your own files, so this should be a no-op every time. If it is not, you have touched
   someone else's file — stop and tell A.
4. **Each person works in their own clone of the same mirror.** All three run `entire enable`
   independently. Checkpoints from all three land on the shared `entire/checkpoints/v1` branch —
   **the checkpoint history is a team artefact**, which is exactly what the judges want to see.
5. **Commit small and often with real messages.** Checkpoint quality is 15 points
   (Guide line 278) and it is scored on whether the checkpoints "preserve useful intent and
   decision context". A commit message saying "wip" wastes a scored artefact. Capture decisions,
   rejected options, failures, and assumptions (Guide line 117).

---

## §6 — Integration conflicts: the complete list

There are exactly **four** places where two people could collide. All four are one-line edits and
all four are pre-assigned to a single owner.

| # | File | Line | Owner | Mitigation |
| --- | --- | --- | --- | --- |
| 1 | `cmd/entire/cli/root.go` | ~198 | **A only** | one `AddCommand` line |
| 2 | `cmd/entire/cli/agent_help_cmd.go` | ~142 | **A only** | one map entry |
| 3 | `cmd/entire/cli/setup.go` | 93 | **C only** | one struct field |
| 4 | `handoff/extractors.go` | — | **A only** | stubbed at M1 (§4.1), never edited again |

**The `extractors.go` trick is the whole design.** A writes every constructor reference at M1 and
puts the stubs in `stubs.go`. B and C never open the wiring file — they implement
`newDeadEndsExtractor()` in `deadends.go` and delete their stub line. No shared file changes twice.

**Non-conflicts, stated so nobody worries:** every `handoff/*.go` file has exactly one owner; the
fixture is B's alone; `go.mod`/`go.sum` are frozen (§3.3).

### 6.1 The one API detail that will bite

`Checkpoint.Summary` is a **pointer and is often nil.** Every extractor that reads it must nil-check
first. This is the most likely source of a panic in the 12:05 demo. C tests it; A's
`packet_test.go` asserts an all-nil-Summary `Input` produces a packet with Notes and no panic.

### 6.2 The second one

`GetCompactTranscriptStart()` returns `(offset int, ok bool)`, and **`ok == false` means legacy,
read from line 0** — not "error". A converts it once in `input.go`; B consumes the converted
fields. Nobody calls the metadata method twice.

---

## §7 — Milestones and integration gates

### M0 — 10:40 · Fork exists, everyone is cloned
**Owner: A.** Exit: all three people have a working clone with `entire enable` done and
`mise run build` green. **Nothing else starts until this is true.**

### M1 — 11:05 · Contract pushed, everyone compiles
**Owner: A.** Exit: `packet.go`, `extractors.go`, `stubs.go`, `handoff.go`, `input.go` on
`main`; `entire handoff` prints checkpoint IDs and three "not implemented" sections.
**Handoff point:** A says *"contract is on main, rebase now."* B and C rebase and their files
compile for the first time.

### M2 — 11:35 · INTEGRATION GATE 1
**Owner: A.** This is the gate that matters — it produces the pre-noon stable state the
Participant Guide requires at 11:45 (lines 138–142).

```bash
git checkout main
git merge --no-ff ws/deadends
git merge --no-ff ws/sections
mise run fmt && mise run lint      # in that order — fmt rewrites, so lint AFTER
mise run test:ci                   # budget 3+ minutes
entire handoff                     # eyeball it
entire handoff --json > handoff-pre-noon.json
```

**If `test:ci` fails at 11:45, revert the offending merge, do not debug it.** A green stable
commit at 11:50 is worth more than a broken complete one.

### M2b — 11:50 · Pre-noon checkpoint (MANDATORY, Guide lines 138–142)
- Commit the stable state; confirm the `Entire-Checkpoint:` trailer is on it.
- Record in the commit message: current intent, architecture, completed work, unresolved work,
  technical risks. **This is a scored artefact** — it is one of the four required checkpoints.
- `entire graph checkpoint <id> --json` → save as the semantic-diff record.
- Push. Save `handoff-pre-noon.json` as the "before" artefact.
- **Everyone closes their agent session** (Guide line 146).

### M3 — 12:00 · THE CURVEBALL — this is the demo, execute it deliberately

**This is 15 points and it is also our product demo. Screen-record steps 1–4.**

1. **12:00** — before opening the card, run `entire handoff` in the working session. Leave the
   markdown on screen. **Start the recording here.**
2. Close the session. Open a **fresh** agent session (Guide line 147 requires this).
3. The fresh agent's first action is our own skill: `entire handoff --json`. It begins the
   curveball with the morning's intent, open items and dead ends already loaded. **No paste.**
4. Run `entire graph impact` before touching the affected area (Guide line 148 — mandatory, and
   it is graph evidence #2).
5. A triages the curveball and splits it three ways within 10 minutes. **Land it as a new
   `Extractor` wherever possible** — one file, one stub line, zero conflicts. That is what the
   contract was for.
6. **Eat in shifts.** The guide gives 12:00–13:00 for "curveball and lunch"; do not stop for an
   hour. One person eats while two work.

### M4 — 14:00 · INTEGRATION GATE 2
Same sequence as M2. Plus:
- Graph evidence #3: final semantic diff of the submitted implementation.
- B walks C through the miner (5 min) — C is the demo owner and must be able to explain it.
- Final checkpoint with the curveball response recorded.

### M5 — 14:50 · SUBMIT
**Hard stop. A submits.** Everything below in §9 must already be true.

---

### 7.3 Testing responsibilities

| Who | Tests what | Runs |
| --- | --- | --- |
| A | citation invariant across all extractors; nil-Summary packet; `--json` shape | `go test ./cmd/entire/cli/handoff/...` |
| B | table test over `testdata/transcript.jsonl`: legacy nil start, repeated head line, grouped errors, success-excluded, bad line skipped | same |
| C | three sections incl. the all-nil-Summary Note; four scaffold cases | `go test ./cmd/entire/cli/...` |
| A | **the full gate** | `mise run check` at M2 and M4 only |

**Do not run `mise run test:ci` while iterating.** It is `go test -tags=integration -race ./...`
over the whole CLI plus the Vogon canary — minutes, not seconds, and three people running it
concurrently will make every laptop unusable. The handoff extractor tests are pure functions over
fixture bytes; they need none of the git machinery and they run in under a second. **Keep them
that way** — it is the reason you can run them every ten minutes.

Test rules that will bite (`BUILD_RULES.md` §2): `t.Parallel()` in every test function and
subtest; `t.Chdir()`/`t.Setenv()` after `t.Parallel()` **panics**, so those stay serial; never
touch the real repo CWD — use `testutil.InitRepo(t, tmpDir)`.
**Never run `mise run test:e2e`** — it makes real paid API calls.

---

## §8 — Cut lines, in the order you take them

Re-baselined to the 15:00 deadline. Take these without debate when the clock says so.

| Time | If | Cut to | Who decides |
| --- | --- | --- | --- |
| 11:00 | B's go/no-go grep found no `status:"error"` blocks | dead ends become `Summary.Friction` text; say so in `BUILDATHON.md` | B reports, A decides |
| 11:20 | dead-end miner noisy or not grouping | ship sections 1/2/5 + friction; B pivots to helping C | A |
| 11:35 | anything is half-done | **merge what is green, revert what is not** — the stable commit is not negotiable | A |
| 12:00 | — | **stop everything; the curveball outranks all remaining features** | all |
| 13:30 | graph section 4 not returning | `--no-graph` becomes the default; section 4 is a plain touched-file list | C |
| 14:00 | MCP tempting | **don't.** Skill was always the primary surface | A |
| — | Databricks | **already cut** (§0.1) | — |

**The three sections that must never be cut:** the curveball response (15 pts), the four required
checkpoints (15 pts), and the three graph artefacts (15 pts). Those are 45 of 100 points and none
of them is a feature — they are process artefacts that cost minutes, not hours. Protect them ahead
of any code.

---

## §9 — Submission checklist (A owns, run at 14:40)

From Participant Guide lines 205–216 and 232–240:

- [ ] Final commit pushed; SHA matches the submission form
- [ ] Project launches from a clean checkout (`mise run build && ./entire handoff`)
- [ ] Four checkpoints open and explain: initial understanding · pre-noon stable · curveball
      response · final implementation
- [ ] Three graph artefacts recorded, including the final semantic diff
- [ ] `BUILDATHON.md` complete, **free of secrets**, using the guide's exact headings
- [ ] Tests covering critical **and curveball** behaviour pass
- [ ] Demo owner (C) can run the critical path and explain B's miner
- [ ] Fallback recording of the 12:00 curveball bootstrap saved locally
- [ ] Fork URL, mirror URL, checkpoint links all open **signed out**
- [ ] Submitted before 15:00 IST

### `BUILDATHON.md` must contain, in this order

1. The problem and the intended user.
2. Demand citations: `cli#408`, `#1125`, `#381`, `#985`, `#296`.
3. **The explicit positioning against the shipped `session-handoff` skill.** This is not optional.
   `cli#381` was closed pointing at it; the person who wrote that reply may be judging.
   *Verified: `session-handoff` is a real shipped official skill —
   `cmd/entire/cli/telemetry/skill_official.go:56`, described in
   `agent/skilldiscovery/match_test.go:32` as "inspect recent sessions or summarize a saved
   session".* Name the three things it does not do: **dead-end mining from `status:"error"`
   transcript blocks, graph-scoped blast radius, and the machine-readable packet `cli#1125` asked
   for by name.**
4. The curveball you received and how you adapted.
5. **The three disclosures**, stated plainly:
   - Claude-Code-only transcripts (`external-agents#67`)
   - heuristic dead-end grouping
   - `dependents_count` is a textual index that can undercount
     (`W_ANALYSIS_BUDGET_EXCEEDED`)
6. The `handoff` classification call: unlisted, read-only, and why.
7. Links to `handoff-pre-noon.json`, the post-noon packet, and the recording.

Disclosing your own limits is scored (Guide lines 198, 280: "limits acknowledged"). Judges find
these in ninety seconds anyway. Being first to name them is worth more than hoping.

---

## §10 — The first ten minutes

Right now, in parallel:

- **A:** fork `entireio/cli` on GitHub → `entire repo mirror create` (India) →
  `entire repo clone` → **announce the URL**.
- **B:** read §4.2 and §3. Sketch `scanToolCalls` on paper. The moment A announces, clone and
  run the go/no-go grep.
- **C:** read §4.3 and §3. The moment A announces, clone, `entire plugin install graph`,
  and capture graph evidence #1.

**Everyone:** the contract in §3 is frozen. You do not need A's file to start writing yours.
