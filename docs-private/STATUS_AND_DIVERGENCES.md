# Status: what was built, what was skipped, where we diverged

Accounting against `IMPLEMENTATION_PLAN.md` (Steps 0–11) and
`IMPLEMENTATION_ROADMAP.md` (M0–M5). Written after the fact from the actual
tree, not from intent.

---

## 1. What exists

**One new command**, `entire handoff` / `entire handoff --json`, producing a
five-section cited packet. All five sections implemented; no stubs remain.

**One new binary**, `cmd/handoff-databricks`, consuming packet JSON on stdin and
writing warehouse rows.

**One managed skill**, installed by `entire enable --handoff-skill`, telling a
fresh agent to run the command before reading any file.

| Area | Files |
| --- | --- |
| Command | `cmd/entire/cli/handoff.go` |
| Package | `handoff/`: `packet.go` `input.go` `build.go` `extractors.go` `intent.go` `openitems.go` `transcript.go` `deadends.go` `surface.go` `stopped.go` `render_md.go` `render_json.go` |
| Tests | `packet_test.go` `input_test.go` `deadends_test.go` `surface_test.go` `transcript_realdata_test.go` — **47 test functions** total |
| Skill | `cmd/entire/cli/setup_handoff_skill.go` |
| Warehouse | `cmd/handoff-databricks/{main,rows,upload}.go` + `upload_test.go`, `databricks/ddl.sql` |
| Docs | `docs/architecture/handoff.md`, two workstream briefs in `docs-private/` |
| Registration | 4 one-line edits: `root.go`, `agent_help_cmd.go`, `setup.go`, `root_test.go` |

Quality gates: `go build ./...` clean, `go vet` clean, `gofmt` clean,
`golangci-lint` clean on all new and changed files, full `./cmd/entire/cli/...`
and `./cmd/handoff-databricks/...` suites green.

**No existing behaviour was changed.** Every pre-existing component is read-only
from our side.

---

## 2. Plan step → outcome

| Step | Planned | Outcome |
| --- | --- | --- |
| 0 | Fork → mirror → clone → enable → go/no-go grep | **Diverged.** No fork or mirror; worked in the existing clone. Go/no-go answered differently — see §3.1 |
| 1 | Walking skeleton, registered, printing | **Done** |
| 2 | Packet type + extractor seam | **Done**, with contract additions — §3.4 |
| 3 | Sections 1, 2, 5 | **Done** |
| 4 | Dead-end miner | **Done** — the differentiator |
| 5 | Section 4, blast radius | **Done**, different mechanism — §3.2 |
| 6 | Renderers + stable commit | **Done** |
| 7 | Managed skill | **Done** |
| 8 | Noon curveball protocol | **Not applicable** — no curveball was issued |
| 9 | MCP tool (optional) | **Not built.** `grep entire_handoff cmd/entire/cli/mcp.go` → 0 |
| 10 | Databricks lane (stretch) | **Partial** — Tier 1 only, §3.5 |
| 11 | `BUILDATHON.md` + submit | **Not done.** File does not exist |

---

## 3. Divergences, with reasons

### 3.1 Go/no-go was answered from committed fixtures, not a live session

The plan called for making a throwaway commit with a deliberately failing tool
call, then grepping the resulting transcript for `"status":"error"`.

Instead the answer came from
`cmd/entire/cli/transcript/compact/testdata/claude_expected2.jsonl` — the compact
writer's own committed expected output, which contains **4 real
`status:"error"` blocks**. This is stronger evidence than a one-off live run,
and `transcript_realdata_test.go` now asserts against it continuously, so the
test fails if the writer format and our parser ever diverge.

### 3.2 `graph diff`, not `graph impact`

The plan said: for each file, run `entire graph impact`.

That cannot work. `impact` **requires `--symbol`**, and what a checkpoint carries
is `FilesTouched` — paths, not symbols. `surface.go` uses `entire graph diff`,
which returns entity-level changes with `dependents_count` and needs no symbol
input. Degenerate entities are suppressed rather than dressed up.

### 3.3 Three factual errors in the source documents

| Claim | Reality |
| --- | --- |
| `IMPLEMENTATION_PLAN.md:37` — build "needs CGO for tree-sitter" | No tree-sitter in `go.mod`. No CGO required |
| Plan code samples use go-git **v5** | This repo is on **v6**. First build failed on the v5 import |
| `ROADMAP §6` — "exactly four" conflict points | **Five.** `root_test.go` holds a second group table; `TestRoot_VisibleCommandsAreGrouped` failed until `handoff` was added to it |

`ROADMAP` also cites `§0.1` for the Databricks cut, and cites `§0` in its header
— **§0 does not exist in that file.** `BUILD_RULES.md`, `APPLICATION_ANSWER.md`,
`IDEAS.md`, and `docs/snapshot-format.md` are referenced throughout and none
exist, which is why both workstream briefs inline their rules.

### 3.4 The frozen contract grew three fields

`ROADMAP §3` froze `Input`. We added `Listed`, `Unreadable`, and `Truncated`
after finding that `Load` silently dropped unreadable checkpoints — a repo with
100 unhydrated remote checkpoints reported "no checkpoints in range",
indistinguishable from an empty repo. The freeze existed to protect parallel
work; since the work ended up sequential, nothing was broken by the change.

### 3.5 Databricks was un-cut, then only partly built

`ROADMAP:641` marked Databricks cut. It was reinstated on your instruction, and
the real spec turned out to be `IMPLEMENTATION_PLAN.md` Step 10 all along.

| Tier | Status |
| --- | --- |
| Delta ingest, keyed `(repo_key, checkpoint_id, session_id)` | **Built** |
| `file://` offline fallback | **Built** (not in the plan; added so the demo needs no workspace) |
| Vector Search over `dead_ends`/`intent` | **Not built** — `ddl.sql` creates only a source view |
| `entire handoff --ask "<task>"` | **Not built** |
| MLflow eval over ~20 labelled pairs | **Not built** |

Transport also diverged: the plan did not specify one; we chose Files API +
`COPY INTO` over generated `INSERT`s so no packet text ever enters a SQL string.

### 3.6 Built solo, not by three people

The roadmap's whole parallelisation design — frozen contract, `stubs.go`, one
owner per file, `ws/spine` / `ws/deadends` / `ws/sections` branches — was built
for three people. The two workstream briefs were written and are accurate, but
no second or third person picked them up. Everything landed on `main` in one
sequence, and `stubs.go` was deleted once every extractor was real.

---

## 4. Two bugs found by running it for real

Both were invisible to the fixtures and only appeared against a repo whose
checkpoints came from upstream.

**Silent skip.** `Load` dropped any checkpoint whose `ReadSessionContent` failed
and said nothing. 100 checkpoints present, "no checkpoints in range" reported —
wrong in exactly the fresh-clone case the command exists for.

**Unbounded walk.** Only successes counted toward `--limit`, so a run with no
readable checkpoints attempted every entry, each a network fetch. Measured:
**over 10 minutes** before being killed. Now bounded by `LoadBudget` (10s) or
`minLoadAttempts`, returning in **13 seconds** and saying it stopped early.

---

## 5. Open, in priority order

1. **Capture is working; six early commits are permanently uncaptured.** Entire's
   hooks were installed all along, but every invocation skipped with *"not
   installed or not on PATH"* — the binary only ever existed at `/tmp/entire`.
   Fixed by installing to `/usr/local/bin/entire`. Capture resumed on the **next
   turn**, with no session restart: Claude Code runs its hooks as a per-turn
   shell line, not as config loaded once at startup.

   Current state: session `a1ae8c2c…` active, shadow branch
   `entire/aaf0971-e3b0c4` present, and `aaf097182` carries
   `Entire-Checkpoint: 01M1TRGYGTEBC9AXARF0P0Y156`, which resolves under
   `entire checkpoint explain`. **1 real checkpoint exists**; every commit from
   here produces one. The six commits made before the fix have no session data
   behind them and cannot be reconstructed. Four checkpoints are a scored
   artefact, so three more are still needed.
2. **`BUILDATHON.md` does not exist.**
3. **Unpushed commits** — push so the checkpoint syncs to `origin`.
4. **`surface.go` is untested against live graph output** — covered only by an
   injected fake, so the real `entire graph diff` field names are an informed
   guess. Mismatch degrades to the touched-file list rather than breaking.
5. **No graph evidence artefacts captured** — the roadmap requires three.
6. MCP tool, Vector Search, `--ask`, MLflow: not built.

---

## 5a. A correction worth recording

An earlier version of this document stated that no checkpoints existed and that
a fresh session was required to start capturing. **Both were wrong.** The claim
came from inferring how hooks load rather than checking: "No sessions" was
observed moments after the PATH fix and read as proof that a running process
could not pick up hooks mid-flight. In fact Claude Code executes its hook line
every turn, so capture began on the very next one. The lesson is the one this
product exists to serve — verify against the artefact, do not reason from
memory about what the system probably does.

---

## 6. Honest positioning

Against the shipped `session-handoff` skill, three things here are genuinely
new: **dead-end mining from `status:"error"` transcript blocks** (no CLI command
or exported helper surfaces that data today), **line-level citations** that make
each claim resolvable via `entire checkpoint explain`, and **a stable
machine-readable packet**.

The warehouse lane is real but Tier 1 only. Do not describe it as retrieval or
semantic search — it is ingest plus a view shaped for an index that was never
built.
