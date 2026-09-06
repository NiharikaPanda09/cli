# Continuity — step-wise implementation plan

**Scope:** `entire handoff` on a fork of `entireio/cli`, Track E1, per `APPLICATION_ANSWER.md`.
**Clock:** build window 09:00–16:00 IST, Sunday 6 September 2026. Curveball reveal 12:00. Code freeze 16:00.
**Reference:** `IDEAS.md` §2 (checkpoint layout), §5 (E1-1); `btw_buildathon_2026.md` §6, §8, §10.

---

## Correction to carry into the build (found by reading the code, not the dossier)

`IDEAS.md:294-310` says the dead-end miner can parse `transcript.jsonl` with
`compact.BuildCondensedEntries`. **It cannot.** `cmd/entire/cli/transcript/compact/parse.go:88-105`
builds each `tool` entry from `block["name"]` and `block["input"]` only — it never reads
`block["result"]`, so `result.status` is discarded before it reaches a caller. `CondensedEntry` is
`{Type, Content, ToolName, ToolDetail}` (`parse.go:10-15`): there is no status field to read, and no
line index to slice by either.

The `result` object *is* on disk — `compact.go:476` writes `blocks[idx]["result"]` and
`buildToolResult` (`compact.go:488-507`) sets `Status: "error"` when `tr.isError`. So Step 4 parses
the v1 line shape directly in our own package. Two consequences, both good:

- It is ~40 lines (`{v, agent, cli_version, type, ts, id, content[]}` → `content[].result.status`),
  and it keeps line offsets, which we need anyway for `compact_transcript_start` slicing.
- It is the concrete answer to "isn't this a wrapper?" — the miner reads transcript internals no CLI
  command and no exported helper exposes today.

Do **not** spend the morning extending `compact` to export results: `toolResultJSON` is unexported,
the package is shared by seven agent sniffers, and a change there drags upstream tests with it.

---

## Step 0 — Ground (09:15 → 09:35)

Rule Q1: implementation starts only after 09:00 in a **new fork**. Everything below happens there.

1. Fork `entireio/cli` → mirror (India region) → clone. `cd cli`.
2. `mise run build` — needs CGO for tree-sitter. If it fails, fix it now, not at 11:00.
3. `entire enable`, `entire agent add claude-code`, `entire graph init-agents`.
4. Make one throwaway commit. **Verify before writing feature code:**
   - `git log -1 --format=%B` shows an `Entire-Checkpoint:` trailer;
   - `entire checkpoint list` shows the checkpoint;
   - `git cat-file -p refs/entire/checkpoints/<shard>/<id>` shows `0/transcript.jsonl`;
   - `grep -c '"status":"error"' <that transcript>` returns > 0 after one deliberately failing tool
     call (e.g. `cat /nope`). **This grep is the go/no-go for the whole product.** If Claude Code
     writes no `status:"error"` blocks here, Steps 4 collapses to `friction`/`learnings` text and
     you say so in `BUILDATHON.md`.
5. Commit the untouched fork state so there is a clean "before" checkpoint for judging.

---

## Step 1 — Walking skeleton, registered and printing (09:35 → 10:00)

Mirror the `recap` shape: thin cobra file at the top level, logic in a sibling package.

**New files**
- `cmd/entire/cli/handoff.go` — `newHandoffCmd()`, `handoffFlags`, `runHandoff(ctx, out, errW, f)`.
- `cmd/entire/cli/handoff/` — package `handoff`.

**One edit** — `cmd/entire/cli/root.go:197` region:
```go
cmd.AddCommand(inGroup(newHandoffCmd(), groupSessions))
```

**Flags** (`handoff.go`, copying `recapFlags` at `recap.go:26-35`):
```go
type handoffFlags struct {
    session    string // session id; default = latest
    checkpoint string // start from this checkpoint id
    format     string // "md" (default) | "json"
    limit      int    // max checkpoints walked, default 10
    noGraph    bool   // skip section 4
}
```

**Store open** — verbatim from `resume.go:198`:
```go
stores, err := checkpoint.Open(ctx, repo, checkpoint.OpenOptions{
    BlobFetcher: FetchBlobsByHash,
    RefFetcher:  FetchCheckpointRef,
    ReadRemotes: strategy.CheckpointReadRemotes(ctx),
})
```
`stores.Persistent` satisfies `api/checkpoint.PersistentStore` → `List(ctx)`, `Read(ctx, id)`,
`ReadSessionMetadataAndPrompts(ctx, id, i)`, `ReadSessionContent(ctx, id, i)`
(`api/checkpoint/interfaces.go:13-24`).

**Exit criterion:** `entire handoff` prints the resolved checkpoint IDs and session count. Commit.

---

## Step 2 — The packet type and the extractor seam (10:00 → 10:15)

Five sections behind one interface — this is what makes a curveball that adds a section a one-file
change. Do not skip it to save fifteen minutes; it is the curveball insurance policy.

`cmd/entire/cli/handoff/packet.go`:
```go
type Citation struct {
    CheckpointID string `json:"checkpoint_id"`
    SessionIndex int    `json:"session_index"`
    Line         int    `json:"line,omitempty"` // transcript.jsonl offset, dead ends only
}

type Item struct {
    Text  string     `json:"text"`
    Cites []Citation `json:"cites"`
}

type Section struct {
    Name  string `json:"name"`
    Items []Item `json:"items"`
    Note  string `json:"note,omitempty"` // e.g. "graph degenerate: no_dependents"
}

type Packet struct {
    Repo        string    `json:"repo"`
    GeneratedAt time.Time `json:"generated_at"`
    Head        string    `json:"head_checkpoint_id"`
    Sections    []Section `json:"sections"`
}

type Input struct { // everything an extractor may read; assembled once
    Checkpoints []Checkpoint // id, summary, per-session metadata, prompts, transcript bytes
    Graph       GraphRunner  // interface, so tests inject a fake
}

type Extractor interface {
    Name() string
    Extract(ctx context.Context, in Input) (Section, error)
}
```
**Invariant to enforce in a test:** every `Item` has at least one `Citation`. "Every line carries a
checkpoint ID" is the claim in the application answer; make it mechanical.

An extractor that fails returns its error, and `runHandoff` renders the section as
`(unavailable: <err>)` rather than aborting. Partial packets are the point.

---

## Step 3 — Sections 1, 2, 5 (10:15 → 10:45)

All three read `Metadata.Summary` — `{Intent, Outcome, Learnings, Friction, OpenItems}` at
`api/checkpoint/metadata.go:596-600`. No transcript parsing yet.

1. **Intent** (`intent.go`) — `summary.intent` per checkpoint, newest first, near-duplicate lines
   dropped (normalize case/whitespace; do not get clever).
2. **Open items** (`openitems.go`) — union of `summary.open_items`, minus any item whose text names a
   path that appears in a *later* checkpoint's `files_touched` (`metadata.go:405`). Mark those
   `likely closed` in a separate list rather than deleting them — a silently dropped promise is the
   exact failure the product claims to fix.
3. **Where it stopped** (`stopped.go`) — last checkpoint's `initial_attribution`, `token_usage`,
   `session_metrics`, plus HEAD's `Entire-Checkpoint` trailer (`git log -1`).

**Guard:** `open_items` is only populated when summaries were generated. When every checkpoint has an
empty `Summary`, emit `Note: "no session summaries in range — run with more checkpoints"`.

Commit. At this moment the project is already demoable without Step 4.

---

## Step 4 — The dead-end miner (10:45 → 11:30) — **this is the product**

`cmd/entire/cli/handoff/transcript.go` (own parser, per the correction above):

```go
type toolCall struct {
    Line   int
    Name   string
    Input  map[string]any
    Status string // "success" | "error" | ""
    Output string
}
func scanToolCalls(transcript []byte, from int) ([]toolCall, []string, error)
```
- Split on `\n`, `json.Unmarshal` each into `{Type string; Content json.RawMessage}`; skip blank and
  unparseable lines (do not fail the command on one bad line).
- For `type == "assistant"`, unmarshal `Content` into `[]map[string]json.RawMessage`; for blocks with
  `type == "tool_use"`, read `name`, `input`, and `result.{status,output}`.
- Keep the assistant `text` block that follows an errored call in the same or next line — that text
  is *why the approach was abandoned*, and it is the highest-value string in the packet.

**Slicing (the one trap, `IDEAS.md:120-129`):**
- `Metadata.GetCompactTranscriptStart() (int, bool)` (`metadata.go:503`) indexes `transcript.jsonl`.
  **`ok == false` is the legacy path → start at line 0.**
- `Metadata.GetTranscriptStart()` (`metadata.go:491`) indexes raw `full.jsonl`. **Different
  coordinate system — never use it against `transcript.jsonl`.**
- Tolerate **one repeated line at the head**: the boundary rounds toward inclusion when a streaming
  message straddles it. Dedupe by `(line.id, block index)`.

**Grouping** (`deadends.go`): key on `input.command` for Bash, `input.file_path` for edits, else tool
name; collapse to one entry per key with a count, first line offset, the error output truncated to
~200 chars, and the following assistant text. Order by count desc. Cap at 12 entries.

**Tests** (`deadends_test.go` + `testdata/transcript.jsonl`): hand-write a fixture with a legacy nil
start, a repeated head line, two errors on the same command, one on a file path, and one success that
must not appear. Table test, `go test ./cmd/entire/cli/handoff/...`.

**MVP scope disclosure:** Claude Code only. `compact.Compact` sniffs OpenCode, Gemini, pi, Codex,
Copilot and Droid before the Claude/Cursor path, and `ea#67` reports amp/goose/kilo/kiro/qwen produce
no usable transcript. Say this in `BUILDATHON.md` and in the demo, in one sentence.

---

## Step 5 — Section 4, blast radius (11:30 → 11:45)

`surface.go`: union `files_touched` across the range → for each file, `entire graph impact --repo .
--symbol <name> --format json`; read `degenerate` / `degenerate_reason` and **suppress** degenerate
entries instead of dressing them up. Shell out via `exec.CommandContext` behind the `GraphRunner`
interface so tests inject a fake and `--no-graph` is a one-line bypass.

**Disclose:** `dependents_count` is a textual identifier index, not a graph query
(`entire-graph/internal/sem/dependents.go`); it can emit `W_ANALYSIS_BUDGET_EXCEEDED` and undercount,
and it is suppressed for `added` entities (`IDEAS.md` §1). A judge who opens that file will find it in
ninety seconds — so put it in `BUILDATHON.md` first.

If graph is not producing output by 11:40: `--no-graph` becomes the default, section 4 falls back to a
plain touched-file list, and you keep moving. **This is a hard cut line.**

---

## Step 6 — Render, verify, stable commit (11:45 → 12:00) — Curveball Phase 1

1. `render_md.go` and `render_json.go`; `--json` is the machine-readable packet `cli#1125` asked for
   by name — keep it a stable, documented shape.
2. `mise run fmt && mise run lint && mise run test`.
3. **Stable commit before 12:00.** Confirm the `Entire-Checkpoint:` trailer on it and run
   `entire graph checkpoint <id> --json` as the semantic-diff record. Push.
4. Save `entire handoff --json > handoff-pre-noon.json` as the "before" artefact for judging.

---

## Step 7 — The Entire-managed skill (12:00 window, in parallel with the curveball)

Ship as a **skill, not MCP**: `cli#292` is closed `wontfix` in favour of skills. Copy the
`scaffoldSearchSkill` → `searchSkillTemplate(agentName)` pair exactly.

**Four-file change:**
1. `cmd/entire/cli/setup_handoff_skill.go` — `scaffoldHandoffSkill(ctx, ag)` and
   `handoffSkillTemplate(agentName) (relPath string, content []byte, ok bool)`, returning
   `filepath.Join(".claude", "skills", "entire-handoff", "SKILL.md")` for `agent.AgentNameClaudeCode`
   (pattern: `setup_agent_help_skill.go:89-100`). Marker const
   `ENTIRE-MANAGED HANDOFF SKILL v1`, checked by `isManagedHandoffSkill`, written through
   `writeManagedScaffold(root, relPath, content, isManagedHandoffSkill)` — that helper already gives
   you created/updated/unchanged/skipped-conflict semantics for free.
2. `setup.go` — one `HandoffSkill bool` field on `EnableOptions` (`setup.go:70`).
3. Wire `setupOptionalHandoffSkill` / `...ForNames` beside the search-skill calls.
4. `setup_handoff_skill_test.go` — clone the four cases in `setup_search_skill_test.go`
   (created, unchanged, updated, skipped-conflict).

**Skill body:** one instruction — *run `entire handoff --json` first, before reading any file* — plus
the packet's section names. That is the "fresh agent bootstraps in a single call" claim, made real.

---

## Step 8 — Noon Curveball protocol (12:00 → 12:20) — the demo

This is the differentiator; execute it deliberately and narrate it.

1. At 12:00, run `entire handoff` in the working session. Keep the markdown on screen.
2. **Close the session.** Open a **fresh** agent session (rule §6 Phase 2 requires this).
3. The fresh agent's first action is the skill: `entire handoff --json`. It starts working on the
   curveball with the morning's intent, open items and dead ends already loaded. No paste.
4. Screen-record steps 1–3. That recording *is* the curveball answer and the product demo at once.
5. Land the curveball requirement as a new `Extractor` where possible — one file, one registration.

---

## Step 9 — Second surface: MCP tool (12:20 → 13:00, optional)

Two edits in `cmd/entire/cli/mcp.go`, exactly as `entire_status` buffers `runStatusJSON`:
1. `mcpToolDefs()` (~`mcp.go:184`): add `entire_handoff` with an input schema of `{session, limit}`.
2. `handleMCPToolCall` (~`mcp.go:208`): **widen the hardcoded `Arguments struct { Command string }`**
   to carry the new fields, then `runHandoffJSON(ctx, &buf)` → `mcpToolTextResult(buf.String())`.

Skill first, MCP second — the ordering is the positioning argument, so do not invert it under time
pressure.

---

## Step 10 — Databricks lane (13:00 → 15:30, stretch)

Only after Steps 1–8 are committed and green.
1. `cmd/handoff-databricks/` writes packets to a **Delta** table keyed on
   `(repo_key, checkpoint_id, session_id)` — never `repo_key` alone, which is not unique for
   `local/<basename>` (`docs/snapshot-format.md`).
2. **Vector Search** over the `dead_ends` and `intent` columns; `entire handoff --ask "<task>"`
   retrieves *"three sessions ago someone tried this and it failed because X."*
3. **MLflow** eval run over ~20 hand-labelled pairs, so retrieved-dead-end relevance is a number in
   the submission rather than a claim.
4. No credentials in the repo, in markdown, or in the video (§10 Security Mandate). Env vars only.

---

## Step 11 — Submit (15:30 → 16:00)

`BUILDATHON.md` must contain, in this order: the problem; the demand citations (`cli#408`, `#1125`,
`#381`, `#985`, `#296`); **the explicit positioning against the shipped session-handoff skill** —
`cli#381` was closed pointing at it, and the person who wrote that reply may be judging — naming the
three things it does not do (dead-end mining, graph-scoped blast radius, machine-readable packet);
the curveball you received and how you adapted; and the three disclosures (Claude-Code-only
transcripts, heuristic dead-end grouping, textual `dependents_count`). Link the pre-noon and post-noon
packets. Submit by 16:00 — the freeze is hard.

---

## Cut lines, in the order you take them

| Time | If not working | Cut to |
| --- | --- | --- |
| 11:00 | dead-end miner noisy or no `status:"error"` blocks | ship sections 1/2/5 + `friction` text; say so |
| 11:40 | graph impact not returning | `--no-graph` default; plain touched-file list |
| 12:00 | anything half-done | commit the stable state *first*, then continue |
| 13:00 | MCP fiddly | skill only — it was always the primary surface |
| 15:00 | Databricks lane not landing | drop it; a finished E1 project still ships |

## Definition of done (by 16:00)

- `entire handoff` and `entire handoff --json` run green in a fresh clone of the fork.
- `mise run check` passes.
- Table tests over a committed transcript fixture cover: legacy nil `compact_transcript_start`, the
  repeated head line, grouped errors, and the every-item-has-a-citation invariant.
- The managed skill scaffolds on `entire enable` and a fresh agent bootstraps from it in one call.
- `BUILDATHON.md`, the pre/post-noon packets, and the recording are in the repo.
