# `entire handoff` — Technical Documentation

**Version:** v1 (Buildathon build, 6 September 2026)
**Repository:** `github.com/NiharikaPanda09/cli` (fork of `entireio/cli`), branch `main`
**Track:** Entire Track 1 — Checkpoint-Native Developer Experience

---

## Table of contents

1. [What this is](#1-what-this-is)
2. [Installation](#2-installation)
3. [Command reference](#3-command-reference)
4. [The packet: the six sections](#4-the-packet-the-six-sections)
5. [JSON schema](#5-json-schema)
6. [Architecture](#6-architecture)
7. [The dead-end miner](#7-the-dead-end-miner)
8. [The privacy boundary](#8-the-privacy-boundary)
9. [The Databricks lane](#9-the-databricks-lane)
10. [The managed agent skill](#10-the-managed-agent-skill)
11. [Testing](#11-testing)
12. [File map](#12-file-map)
13. [Known limitations](#13-known-limitations)
14. [Troubleshooting](#14-troubleshooting)

---

## 1. What this is

`entire handoff` reads a repository's Entire Checkpoints and emits a short,
**fully cited** briefing for whoever picks the work up next — the same developer
tomorrow morning, a teammate in another timezone, or a fresh agent session with
an empty context window.

Every line in the output carries the checkpoint ID it came from, so any claim can
be traced back with `entire checkpoint explain <id>`. A briefing you cannot verify
is a rumour with better formatting.

### The problem it solves

Git records what *changed*. It cannot record what was *attempted and abandoned*.
So every new session re-tries an approach that already failed, re-asks a question
answered yesterday, and silently drops the three things the last session promised
to come back to. The cost is invisible and it is paid again in full, every time.

The three highest-value outputs here are impossible to produce from a git diff:

| Output | Source | Why git cannot do it |
| --- | --- | --- |
| **Dead ends** | `result.status == "error"` blocks in the checkpoint's `transcript.jsonl` | No git artifact records that a tool call failed, or what the agent said afterwards about giving up |
| **Intent / open items** | Checkpoint `Summary` fields | Exists because Entire captured the session, not because someone wrote a good commit message |
| **Citations** | Checkpoint IDs | Git has no session provenance to cite |

---

## 2. Installation

Requires **Go 1.26+**. No CGO, no tree-sitter dependency.

```bash
go build -o ~/.local/bin/entire ./cmd/entire
export PATH="$HOME/.local/bin:$PATH"     # persist this in ~/.bashrc
entire version
```

> **The `$PATH` step is not optional.** Entire's git hooks resolve the binary via
> `command -v entire`. If it is not on `$PATH`, every hook silently no-ops and
> **no checkpoints are recorded at all** — see [Troubleshooting](#14-troubleshooting).

Optional, for the blast-radius section:

```bash
entire plugin install graph      # entire-graph v0.4.0
```

Optional, for the Databricks ingest binary:

```bash
go build -o ./handoff-databricks ./cmd/handoff-databricks
```

---

## 3. Command reference

```
entire handoff [flags]
```

Build a handoff packet from recent checkpoints. **Read-only** — it writes nothing
to the repository.

| Flag | Default | Description |
| --- | --- | --- |
| `--json` | `false` | Shorthand for `--format json` |
| `--format <md\|json>` | `md` | Output format. `md` is human-readable; `json` is the stable machine shape |
| `--limit <n>` | `10` | Maximum checkpoints to read, walking newest-first |
| `--checkpoint <id>` | *newest* | Start here instead of at the newest checkpoint |
| `--session <id>` | *all* | Only read checkpoints belonging to this session ID |
| `--no-graph` | `false` | Skip the graph-derived blast-radius section |
| `--ask "<task>"` | *(empty)* | Search the team's history for prior work on this task (requires Databricks Vector Search — see §9) |

### Classification

Registered as **read-only, unlisted** in `agentHelpClassification`
(`agent_help_cmd.go:147`). Unlisted is the safe default for a new command; the
managed skill (§10) is how a fresh agent discovers it, so nothing is lost.

### Examples

```bash
entire handoff                       # markdown briefing, last 10 checkpoints
entire handoff --json                # machine-readable packet
entire handoff --limit 20            # reach further back
entire handoff --no-graph            # skip blast radius (faster; no plugin needed)
entire handoff --session 01M1TS1K...  # one session only
entire handoff --ask "fix the auth bug"   # cross-repo prior art

entire checkpoint explain <checkpoint_id>  # follow any citation
```

### Output guarantees

- **No TTY dependence.** No pager, no colour, no width probing. Output is byte-identical
  piped and interactive, because the primary reader is an agent with no terminal.
- **No interactive prompts.** No pickers, no `huh` forms, no Bubble Tea. Every selection
  is a flag.
- **Never fails on partial data.** A section that cannot do its job returns zero items
  and a stated reason. A packet that refuses to print because the graph was down is
  worse than useless to whoever needs orienting.

---

## 4. The packet: the six sections

| # | Section name (JSON) | Heading (markdown) | Source | Optional? |
| --- | --- | --- | --- | --- |
| 1 | `intent` | What we were trying to do | `Summary.Intent` per checkpoint | always |
| 2 | `open_items` | Still open | `Summary.OpenItems`, unioned | always |
| 3 | `dead_ends` | Already tried — don't repeat | `transcript.jsonl` error blocks | always |
| 4 | `surface` | Blast radius | `entire graph diff` | skipped by `--no-graph` |
| 5 | `prior_art` | *(name as-is)* | Databricks Vector Search | only when configured **or** `--ask` given |
| 6 | `stopped` | Where it stopped | newest checkpoint's metadata | always |

### 1. `intent`

Lists what each session set out to do, newest first. Near-duplicates are dropped
(case and whitespace normalised only) because consecutive checkpoints in one
sitting routinely share an intent, and repeating it three times pushes older,
more informative entries out of view.

Deliberately *not* cleverer than that — stemming or fuzzy matching would silently
merge two genuinely different intents, and losing one is worse than showing both.

### 2. `open_items`

Unions open items across the range and marks the ones a later checkpoint probably
resolved — by checking whether the item text names a file touched by a *newer*
checkpoint (matched on full path **and** base name, since an open item usually
says `recap_test.go`, not the repo-relative path).

**Design rule:** an item is annotated `(likely closed)`, **never deleted**. A
dropped promise is the exact failure this product exists to fix, so a heuristic is
allowed to add a hint and never to remove a line. The reader keeps the final say.

### 3. `dead_ends`

The core of the product. See [§7](#7-the-dead-end-miner).

### 4. `surface`

Runs `entire graph diff --repo . --format json` and ranks changed entities by
`dependents_count`. Degenerate entities are **suppressed**, not dressed up, and the
count of suppressed entities appears in the section note.

Three graceful degradations, in order:
1. Graph binary unavailable → `Note: "graph unavailable: <err>"`
2. Output not valid JSON → `Note: "graph output was not valid json"`
3. No non-degenerate entities → falls back to the plain touched-file list

Every path carries the standing caveat: *`dependents_count` is a heuristic textual
index and can undercount.*

### 5. `prior_art`

Cross-repo, cross-developer retrieval over the team's history. Present **only**
when `DATABRICKS_HOST` / `DATABRICKS_TOKEN` / `DATABRICKS_VECTOR_INDEX` are all set,
or when `--ask` is given. Query text is the `--ask` string, or the two newest
intents when `--ask` is empty. See [§9](#9-the-databricks-lane).

### 6. `stopped`

A position report, not a history — it reads the **newest checkpoint only**. It draws
on metadata rather than the summary on purpose, so it still says something useful
when no summary was generated. That is the common case for an interrupted session,
which is exactly when a handoff matters most.

Emits, when available: outcome, HEAD checkpoint ID, last touched files (capped at 8
with a `… +N more` marker), agent attribution percentage, turn count, token usage.

---

## 5. JSON schema

`--json` is a **published interface**. It is what a fresh agent reads to bootstrap
and what the Databricks ingest lane consumes on stdin. *Adding* a field is safe;
renaming or removing one is a breaking change for both consumers.

```jsonc
{
  "repo": "gh/owner/name",              // or "local/<basename>" with no origin remote
  "generated_at": "2026-09-06T07:10:00Z",
  "head_checkpoint_id": "01M1TS1K9VQC4BW89ASRD24PBE",  // "" if HEAD has no trailer
  "sections": [
    {
      "name": "dead_ends",
      "items": [
        {
          "text": "go build ./... — failed 3 times: undefined: handoff.Load (then: switching approach)",
          "cites": [
            {
              "checkpoint_id": "01M1TS1K9VQC4BW89ASRD24PBE",
              "session_index": 0,
              "line": 412          // transcript.jsonl offset; dead_ends only, omitted elsewhere
            }
          ]
        }
      ],
      "note": "showing the 12 most repeated of 19 dead ends",  // omitted when empty
      "incomplete": true                                        // omitted when false
    }
  ]
}
```

### Normalisation guarantees

`RenderJSON` forces `sections` to a non-nil array and every section's `items` to a
non-nil array. A `null` where an array was expected is the classic way a downstream
parser dies on the one packet that happened to have an empty section.

### Field notes

- **`cites` is never empty.** Enforced mechanically by `TestEveryItemIsCited` across
  all extractors — an uncited item is a bug, not a style preference.
- **`note`** carries the reason a section is empty or truncated. An empty section
  with a note is a *finding*, not a failure.
- **`incomplete`** means the range had unreadable, truncated, or redacted data. It
  never means the section is wrong — only that it may not be the whole picture. See
  [§8](#8-the-privacy-boundary).

---

## 6. Architecture

```
Entire checkpoint store  ──▶  handoff.Load  ──▶  Input (newest-first)
                                                   │
              ┌──────────┬──────────┬──────────┬───┴──────┬───────────┐
              ▼          ▼          ▼          ▼          ▼           ▼
           intent   open_items  dead_ends   surface    prior_art   stopped
                                            (graph)   (opt-in only)
              └──────────┴──────────┴──────────┴──────────┴───────────┘
                                     ▼
                              Packet (every item cited)
                                     │
                     ┌───────────────┴───────────────┐
                     ▼                               ▼
              render_md (human)              render_json (machine)
                                                     │
                                    ┌────────────────┴──────────────┐
                                    ▼                               ▼
                          managed skill (fresh agent          handoff-databricks
                          runs it before reading files)       (Delta ingest)
```

### Three invariants

1. **Every item carries a citation.** Enforced by a test, not by convention.
2. **Failure is a `Note`, not an error.** One extractor failing never fails the
   packet — its section renders as `(unavailable: …)` and the rest still print. An
   agent that gets four good sections is oriented; one that gets an error message
   is not.
3. **Extractors never touch the store.** Everything is loaded once into `Input`, so
   each section is a pure function over a struct. That is why the tests run in
   milliseconds with no git repository at all.

### The extractor seam

```go
type Extractor interface {
    Name() string
    Extract(ctx context.Context, in Input) (Section, error)
}
```

Wiring lives in exactly one place — `allExtractors()` in `extractors.go` — which
names every extractor once, in packet order. Adding a section (including one a
curveball demands) is **a new file plus one line**. No shared file is edited twice,
so parallel workstreams cannot conflict during integration.

### Bounded loading

`handoff.Load` walks the store newest-first under three simultaneous bounds, so a
large or partially-fetched store cannot hang the command:

| Bound | Value | Constant |
| --- | --- | --- |
| Wall clock | 10s | `LoadBudget` |
| Read attempts | `max(limit × 3, 20)` | `loadAttemptScale`, `minLoadAttempts` |
| Checkpoints collected | `--limit` | `DefaultLimit = 10` |

Per-checkpoint failures are **skipped, not fatal** — one unreadable checkpoint
(unpushed, pruned, or written by a newer CLI) must not deny the caller the other
nine. Skipped checkpoints are counted in `Input.Unreadable` and reported in the
section note, never swallowed.

`--checkpoint` with an unknown ID yields the **full** list rather than an empty one:
showing everything is a recoverable surprise, showing nothing looks like data loss.

---

## 7. The dead-end miner

This is the section no other tool produces, and the reason the package parses
transcripts itself.

### Why a custom parser

`transcript/compact.BuildCondensedEntries` **cannot** be used. It builds each tool
entry from `block["name"]` and `block["input"]` only, never reading
`block["result"]` — so `result.status` is discarded before any caller sees it, and
`CondensedEntry` has no status field to read and no line index to slice by.

The `result` object *is* on disk (written by `compact.go`'s `buildToolResult`, which
sets `Status: "error"` when `tr.isError`). So `transcript.go` parses the v1 JSONL
line shape directly:

```
{v, agent, cli_version, type, ts, id, content[]}  →  content[].result.status
```

Extending `compact` to export results was rejected deliberately: `toolResultJSON` is
unexported and the package is shared by seven agent sniffers, so a change there
drags upstream tests with it.

### The offset trap

Two metadata accessors index **different files in different coordinate systems**:

| Accessor | Indexes | Use for dead ends? |
| --- | --- | --- |
| `GetCompactTranscriptStart()` | `transcript.jsonl` | ✅ **yes** |
| `GetTranscriptStart()` | raw `full.jsonl` | ❌ **never** |

`ok == false` from `GetCompactTranscriptStart` is the **legacy path** and means
*start at line 0*, not *skip everything*. Resolved exactly once in `loadCheckpoint`
so no extractor can reach for the wrong one.

### Head-line duplication

The compaction boundary rounds toward inclusion when a streaming message straddles
it, so the first line can repeat. Deduped by `(line.id, blockIndex)`.

### Grouping

Failures collapse to one entry per key, ordered by count descending, then by first
line offset. Key resolution, in order:

1. `input.command` (Bash)
2. `input.file_path` (edits)
3. `input.path`
4. `input.pattern`
5. the tool name
6. `"unknown tool"`

Each entry carries: the key, the failure count, the error output (truncated to 200
chars on a word boundary), and — the highest-value string in the packet — the
**trailing assistant text from the same line**, which is *why the approach was
abandoned*. Capped at 12 entries with an overflow note.

### Honest empty states

The section distinguishes three different kinds of nothing, because they mean
completely different things to a reader:

| Condition | Note |
| --- | --- |
| No checkpoints in range at all | `no checkpoints in range` (or the unreadable/truncated detail) |
| Checkpoints present, no transcripts | `no transcripts available in range` |
| Transcripts present, no errors | `no failed tool calls found in range` |

---

## 8. The privacy boundary

Implemented in response to the Noon Curveball (Track 1 — Privacy Boundary). The
card is committed verbatim as `curveball.md`.

**What it exposed.** The feature added immediately before noon — cross-repo
retrieval (`--ask`) — was the violation. It POSTed query text to Databricks Vector
Search, and that text was either the user's `--ask` string typed verbatim or, when
empty, a checkpoint's `Summary.Intent`. Both are prompt-shaped; neither had been
through redaction at that point.

### 1. Egress is redacted

`redact.String` runs over the query **immediately before it enters the HTTP body**
(`globalmemory.go`) — the same call `checkpoint/persistent.go:1219` already makes on
`Summary.Intent`, and one that uses only local scanners, so closing a network
exposure does not open another.

### 2. The ingest half needed no change — and we checked rather than assumed

`cmd/handoff-databricks` never imports the handoff package and never sees
`Checkpoint.Transcript`. It consumes packet JSON on stdin, so the only text it can
send is `Item.Text` — already extractor-derived, already truncated, already redacted
at rest by the checkpoint pipeline.

### 3. Incomplete context is visible, not implied

`Section.Incomplete` is set **centrally in `BuildWith`**, after every extractor
returns, from `Input.hasGaps()`. Setting it centrally rather than per-extractor is
the point: a future section cannot forget to set it.

`hasGaps()` counts two kinds of evidence, and the second is **deliberately narrow**:

- **Range level** — a checkpoint that could not be read at all, or a range we
  stopped reading early. Something we meant to look at is definitely missing.
- **Checkpoint level** — a checkpoint we *did* load that yielded **neither** a
  Summary **nor** a Transcript. A checkpoint should carry at least one, so having
  neither means content that existed when the session ran is not here now.

> **Why not "either field missing"?** That rule sounds safer and is useless. The
> Transcript is Claude-Code-only and the Summary is nil whenever summarization is
> off — so under that rule *every* packet from a Gemini or Codex user carried a
> permanent "incomplete context" banner. A warning that is always on is a warning
> nobody reads, which costs you the one case it exists to announce.

Markdown renders a `⚠ Incomplete context` banner plus a per-heading `(incomplete)`
marker; `--json` carries the boolean.

### What we did not do

We did not remove the retrieval feature. Deleting it would have satisfied the letter
of the constraint and lost the product's cross-repo claim. Redacting at the boundary
keeps the capability and makes the boundary explicit.

---

## 9. The Databricks lane

Databricks provides the one capability that is **structurally impossible** locally:
checkpoints are per-repo and per-clone, so a local packet can only ever describe
*your own* sessions. `prior_art` — *"someone on another repo hit this three sessions
ago and it failed because X"* — is not a feature we chose to host on Databricks; it
is a feature git has nowhere to put.

### Data model — `databricks/ddl.sql`

```
main.default.handoff            VOLUME  — landing zone for NDJSON uploads
main.default.handoff_packets    TABLE   — one row per (section, item, citation)
main.default.handoff_retrievable VIEW   — dead_ends + intent, for the vector index
main.default.handoff_idx        INDEX   — Delta Sync, managed embeddings
```

**Merge key is `(repo_key, checkpoint_id, session_id)` — never `repo_key` alone.** A
repo with no remote gets a synthetic `local/<basename>` key, so two developers' local
clones collide on `local/cli`. Row-level uniqueness additionally needs `section_name`
and `item_index`, because one checkpoint contributes many rows.

**`item_index = -1` marks a section that produced no items**, with `section_note`
carrying the reason it degraded. Those rows are kept deliberately: *"the graph was
degenerate"* is itself a finding, and dropping them makes an empty section
indistinguishable from one that was never run.

### Write path — `cmd/handoff-databricks`

```
packet JSON (stdin)  →  Flatten()  →  EncodeNDJSON()  →  Files API PUT  →  COPY INTO
```

Transport was chosen so that **no packet text ever enters a SQL string**. The
`COPY INTO` statement references a volume path only — asserted by
`TestCopyIntoStatementCarriesNoPacketText`.

| Env var | Required | Purpose |
| --- | --- | --- |
| `DATABRICKS_HOST` | ✅ | Workspace URL (**https only**), or `file:///path` for offline mode |
| `DATABRICKS_TOKEN` | when remote | Bearer token. **Environment only, never a settings file** — those are version-controlled |
| `DATABRICKS_VOLUME_PATH` | when remote | Unity Catalog volume, e.g. `/Volumes/main/default/handoff` |
| `DATABRICKS_WAREHOUSE_ID` | for `COPY INTO` | SQL warehouse that runs the load |
| `DATABRICKS_TABLE` | optional | Defaults to `handoff_packets`; set to `main.default.handoff_packets` to match the shipped DDL |

```bash
# Offline demo — no workspace or token required
DATABRICKS_HOST=file:///tmp/out entire handoff --json | ./handoff-databricks

# Inspect the rows without sending anything
entire handoff --json | ./handoff-databricks -dry-run

# Read from a file instead of stdin
./handoff-databricks -in cmd/handoff-databricks/testdata/packet.json -dry-run
```

Flags are `-in <path|->` and `-dry-run`. Plain-HTTP hosts are rejected
(`TestUploadRejectsPlainHTTPHost`), and path segments are sanitised against
traversal (`TestSanitizeSegmentStripsPathTraversal`).

### Read path — Vector Search

`POST /api/2.0/vector-search/indexes/<index>/query`, filtered to
`{"section_name": "dead_ends"}`, 5 results, 20s timeout.

| Env var | Purpose |
| --- | --- |
| `DATABRICKS_HOST` | Workspace URL (https enforced) |
| `DATABRICKS_TOKEN` | Bearer token |
| `DATABRICKS_VECTOR_INDEX` | e.g. `main.default.handoff_idx` |

The index uses **managed embeddings** (`databricks-gte-large-en`), so no client ever
computes a vector — the CLI POSTs a query string and Databricks embeds it. That is
what keeps the Go side on the standard library with **no SDK and no `go.mod` change**.

**Citation columns are carried through retrieval on purpose.** A retrieval result
without a checkpoint ID is precisely the unverifiable assertion the packet exists to
avoid — rows arriving without provenance are dropped
(`TestVectorSearcherDropsRowsWithoutProvenance`).

Index setup requires Change Data Feed on the source table and a stable `row_id`
primary key; both are handled at the bottom of `databricks/ddl.sql`.

### Degradation

Missing configuration is a note, never an error:

| State | `prior_art` note |
| --- | --- |
| Not configured | `vector search not configured` |
| No intent and no `--ask` | `nothing to search on: no intent in range and no --ask given` |
| Query failed | `vector search unavailable: <err>` |
| No matches | `no prior work found for this task` |

The other five sections are unaffected. Removing Databricks does not break the
command — it removes the cross-repo memory, which is the whole point of having it.

---

## 10. The managed agent skill

```bash
entire enable --handoff-skill --agent claude-code
```

Writes `<root>/skills/entire-handoff/SKILL.md`, where `<root>` is `.claude`
(Claude Code), `.agents` (Codex), or `.gemini` (Gemini). Other agents return
`unsupported` — deliberately narrow, because the packet is only as good as the
transcripts behind it and those are reliably shaped for Claude Code today.

Managed via the standard `writeManagedScaffold` helper, keyed on the marker
`ENTIRE-MANAGED HANDOFF SKILL v1`, giving four outcomes for free:
**created / updated / unchanged / skipped-conflict** (an unmanaged file at that path
is never overwritten).

The skill body is **deliberately one instruction**:

> **Run this first, before reading any file:** `entire handoff --json`

The claim the product makes is *"a fresh agent orients itself in a single call."* A
skill that offers five options does not test that claim; a skill that says "run this
first, before you read anything" does.

Shipped as a **skill, not MCP**, on purpose: `cli#292` is closed `wontfix` in favour
of skills.

---

## 11. Testing

```bash
go test ./cmd/entire/cli/handoff/... ./cmd/handoff-databricks/...
go test -run TestPrivacyBoundary ./cmd/entire/cli/handoff/...   # curveball behaviour
```

**61 tests**, all pure functions over fixture bytes — no git repository, no network,
no temp dirs. That is what keeps them fast enough to run every ten minutes during a
build.

| Group | Covers |
| --- | --- |
| `transcript` / `deadends` | error-only extraction, unparseable-line tolerance, followup text, `compact_transcript_start` honoured, legacy nil offset reads from line 0, repeated-head dedupe, grouping precedence, truncation |
| **real-output** | `TestScanToolCallsAgainstRealCompactOutput`, `TestDeadEndsEndToEndOnRealCompactOutput`, `TestRealCompactOutputExcludesSuccesses` — run against a committed real `transcript.jsonl`, not a hand-written fixture |
| `packet` | **`TestEveryItemIsCited`** (the load-bearing invariant), nil-summary safety, empty range blames checkpoints not summaries, `--no-graph` omits the section, `RenderJSON` never emits null arrays |
| `privacy` | redacted checkpoint still produces usable output *and* flags itself; a complete range is never mislabelled; both query paths assert an AWS-key-shaped secret never reaches outgoing text |
| `input` | attempt bounding, limit, cancelled context, newest-first ordering |
| `globalmemory` | absent unless configured, citations preserved through retrieval, intent-vs-`--ask` precedence, failure is a note, bearer auth, https enforcement |
| `surface` | ranking, degenerate suppression, touched-file fallback, graph failure and malformed JSON both notes, `changes` key accepted |
| `databricks` | row-per-citation, empty sections keep their note, NDJSON shape, local `file://` write, Files API bearer, **no packet text in SQL**, plain-HTTP rejected, path traversal stripped |

### Verification status

Observed on Go 1.26.8 after the Curveball changes: `gofmt -l cmd/entire/cli/handoff/`
clean, `go build ./cmd/entire/...` clean, `go test ./cmd/entire/cli/handoff/...` →
**ok, 0.572s**. The pre-Curveball tree was separately verified green including
`golangci-lint` (recorded in commit `426cc65`).

The Curveball commit itself was written on a machine with **no Go toolchain** and
reviewed by hand rather than compiled. We flag this rather than imply a green run we
did not observe.

---

## 12. File map

### Command surface

| Path | Role |
| --- | --- |
| `cmd/entire/cli/handoff.go` | Cobra command, flags, store wiring, repo/HEAD resolution |
| `cmd/entire/cli/setup_handoff_skill.go` | Managed skill scaffolder + template |
| `cmd/entire/cli/root.go` | Command registration (`groupSessions`) |
| `cmd/entire/cli/agent_help_cmd.go` | `"handoff": {readOnly, unlisted}` classification |
| `cmd/entire/cli/setup.go` | `--handoff-skill` flag on `entire enable` |

### The `handoff` package

| File | Role |
| --- | --- |
| `packet.go` | `Packet` / `Section` / `Item` / `Citation` / `Input` types; `hasGaps()` |
| `input.go` | `Load` — bounded newest-first store walk |
| `build.go` | `BuildWith` — runs extractors, sets `Incomplete` centrally |
| `extractors.go` | The one wiring list |
| `intent.go` · `openitems.go` · `stopped.go` | Summary/metadata sections |
| `transcript.go` | v1 JSONL parser, dedupe, group-key resolution |
| `deadends.go` | Grouping, ranking, truncation, honest empty states |
| `surface.go` | `entire graph diff` runner + fallbacks |
| `globalmemory.go` | Vector Search client, `prior_art`, redacted egress |
| `render_md.go` · `render_json.go` | The two renderers |

### The Databricks lane

| Path | Role |
| --- | --- |
| `databricks/ddl.sql` | Volume, table, view, index, CDF, `row_id` |
| `cmd/handoff-databricks/main.go` | stdin → flatten → encode → upload → `COPY INTO` |
| `cmd/handoff-databricks/rows.go` | Packet → row flattening, NDJSON |
| `cmd/handoff-databricks/upload.go` | Files API, `COPY INTO`, `file://` offline mode |

### Evidence and docs

| Path | Role |
| --- | --- |
| `BUILDATHON.md` | Submission writeup |
| `DEMO.md` · `scripts/demo.sh` | Demo walkthrough |
| `curveball.md` | The Noon Curveball card, verbatim |
| `graph-{1..4}-*.json` | Entire Graph evidence artifacts |
| `cmd/entire/cli/handoff/testdata/` | `transcript.jsonl` (real), `sample_packet.json` (labelled) |

---

## 13. Known limitations

Stated here rather than left to be discovered.

- **Dead-end mining is Claude Code only.** `transcript/compact.Compact` sniffs
  OpenCode, Gemini, pi, Codex, Copilot and Droid before reaching the Claude path, and
  several agents produce no usable `transcript.jsonl` at all. **The other five
  sections work for any agent.**
- **Grouping is a heuristic, not semantic.** Failures group by exact command, then
  path, then tool name. Two commands that mean the same thing spelled differently
  become two entries.
- **`dependents_count` can undercount.** It is a heuristic textual identifier index,
  not a graph query; it emits `W_ANALYSIS_BUDGET_EXCEEDED` and degrades silently
  under budget pressure. `entire-graph`'s own help text calls it "a heuristic
  dependent count."
- **The `surface` section is unit-tested against an injected fake**, so real
  `entire graph diff` field names were an informed guess until the graph artifacts
  were captured. A mismatch degrades to the touched-file list rather than breaking.
- **Vector Search was tested against an `httptest` server**, and — at the time of
  writing — not against a live provisioned index.
- **`graph impact` needs `--symbol`, not a path.** The original plan called for
  running it per touched file; that cannot work, since a checkpoint carries
  `FilesTouched` (paths). `surface` uses `graph diff` instead, which needs no symbol.
- **Six commits carry no checkpoint.** Entire's hooks were installed the whole time
  but silently no-op'd because `entire` was not on `$PATH`. Capture resumed on the
  next turn after the fix. Those six cannot be reconstructed.

### Documentation defects to fix

Two commands in the demo materials do not match the code:

| Location | Says | Should say |
| --- | --- | --- |
| `DEMO.md` §5 | `./handoff-databricks --packet <file>` | `./handoff-databricks -in <file>` — there is no `--packet` flag |
| `scripts/demo.sh` step 5 | tables `entire_handoff_packets`, `entire_handoff_items` | `main.default.handoff_packets` — the `_items` table does not exist; the schema is one flat table |

---

## 14. Troubleshooting

### No checkpoints are being recorded

**By far the most common failure**, and it is silent. Entire's git hooks resolve the
binary via `command -v entire`; if it is not on `$PATH`, every hook prints *"Entire
CLI is enabled but not installed or not on PATH"* and exits.

```bash
sh -c 'command -v entire >/dev/null && echo HOOKS-WILL-RUN || echo BROKEN'

git commit --allow-empty -m "check capture"
git log -1 --format=%B | grep -c "Entire-Checkpoint:"   # must print 1
```

### `no checkpoints in range`

Checkpoints exist but were not fetched locally. Run `entire checkpoint list`; if the
range shows `ListedStub` entries, they are names-only remote-discovery entries with
nothing hydrated behind them.

### `dead_ends` says `no transcripts available in range`

The checkpoints were written by an agent whose transcripts `compact.Compact` cannot
shape — see [§13](#13-known-limitations). The other sections still work.

### `surface` says `graph unavailable`

The `entire-graph` plugin is not installed (`entire plugin install graph`), or the
20s timeout was hit. Use `--no-graph` to skip the section entirely.

### The `⚠ Incomplete context` banner appears

Expected when checkpoints in the range were unreadable, truncated, or carried neither
a Summary nor a Transcript. It does not mean the output is wrong — only that it may
not be the whole picture. Check the section notes for the specific count.

### `DATABRICKS_HOST must use https`

Plain HTTP is rejected outright. Use `https://…` for a workspace, or `file:///path`
for offline mode.

---

*Documentation generated 6 September 2026 for the BTW Buildathon 2026 submission.*
