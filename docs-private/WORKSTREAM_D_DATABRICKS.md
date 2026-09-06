# Workstream D — Databricks lane

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
that already failed, re-asks questions already answered, and silently drops promises the previous
session made. Today the only fix is a human pasting context by hand.

**The product.** `entire handoff` reads the repo's Entire checkpoints and emits a *verifiable* packet
in five sections — **intent, open items, dead ends, surface (blast radius), where it stopped**. The
central claim is that **every line carries a checkpoint citation**, so nothing in the packet is an
unattributable assertion by a language model. A fresh agent runs one command and is oriented.

**Your lane** turns that packet into a durable, queryable, cross-session memory in Databricks. The CLI
answers *"what happened in this repo recently?"*. Your lane answers **"has anyone on this team ever
tried this before, and did it fail?"** — across every repo and every session, not just the local clone.

That retrieval claim is the reason Workstream D exists. It is not analytics decoration.

---

## §2 — Why your work is genuinely independent

This is the most decoupled of the three workstreams, on purpose.

`cmd/handoff-databricks/` is a **separate `main` package**. It consumes **packet JSON on stdin** and
never imports the `handoff` Go package. Consequences:

- You are **not blocked** by the 11:05 contract push. You can build and test right now.
- You will never touch a file another workstream owns, so you cannot cause a merge conflict.
- `cmd/` already contains two binaries (`entire/`, `git-remote-entire/`), so this is an established
  pattern, not a new one you have to justify.

**Write yourself a fixture packet immediately** (§4.0) and develop against it. Do not wait for real
`entire handoff --json` output.

### Where you are NOT independent — stated honestly

| Dependency | Impact | Mitigation |
| --- | --- | --- |
| Real `entire handoff --json` output | Needed only for the **end-to-end demo**, ~11:05 | Use your fixture until then |
| A real Databricks workspace + Unity Catalog volume | Needed for Tiers 2–3 | **Tier 1 must support `DATABRICKS_HOST=file://…`** — see §4.3. Non-negotiable. |
| `entire handoff --ask "<task>"` | Would need a flag in **another person's file** | **Out of your scope.** If Tier 2 lands early, expose a Go function and the integrator wires the flag. Do not edit `handoff.go`. |

---

## §3 — Rules that will fail CI or fail the judges

These are extracted from the repo's `CLAUDE.md`. (`BUILD_RULES.md` is referenced by our planning docs
but **does not exist** — do not go looking for it.)

1. **Standard library only. Do NOT edit `go.mod` or `go.sum`.** No Databricks SDK. Every Databricks
   API you need is HTTP + JSON, so `net/http` and `encoding/json` are sufficient. A `go.sum` conflict
   during late integration is unrecoverable at this clock.
2. **Credentials come from environment variables ONLY.**
   - `DATABRICKS_HOST`, `DATABRICKS_TOKEN`, `DATABRICKS_WAREHOUSE_ID`, `DATABRICKS_VOLUME_PATH`.
   - **Never** put a token in `.entire/settings.json` — that file is version-controlled, so a token
     there ships to everyone who clones the repo.
   - Never in markdown, never on screen in the demo video. Scrub before recording.
3. **`t.Parallel()` in every test function and subtest.** But `t.Setenv()` and `t.Chdir()` **panic**
   if called after `t.Parallel()` — so any test that sets env vars stays serial. You will hit this
   immediately, since your config is env-driven. Prefer injecting a `config` struct over reading env
   inside the function under test.
4. **`mise run fmt` then `mise run lint`** — in that order. `fmt` rewrites files, so a lint result
   from before a `fmt` pass is void.
5. **Never run `mise run test:e2e`** — it makes real, paid API calls.
6. Iterate with `go test ./cmd/handoff-databricks/...`. **Do not run `mise run test:ci` while
   iterating** — it is minutes long, and three people running it at once makes every laptop unusable.
7. **Log only operational metadata** — counts, durations, IDs. Never log packet item text: it is
   derived from prompts and tool output and may contain secrets.
8. **No TTY dependence.** No pager, no prompt, no interactive picker. The binary must work in a pipe.

---

## §4 — The work

### 4.0 — First 10 minutes: the fixture

Create `cmd/handoff-databricks/testdata/packet.json`. **This is the frozen contract** — the exact shape
`entire handoff --json` emits. It will not change.

```json
{
  "repo": "github.com/acme/cli",
  "generated_at": "2026-09-06T10:40:00Z",
  "head_checkpoint_id": "01JQ8Z9K2M3N4P5Q6R7S8T9V0W",
  "sections": [
    {
      "name": "intent",
      "items": [
        { "text": "Add a --json flag to entire recap",
          "cites": [{ "checkpoint_id": "01JQ8Z9K2M3N4P5Q6R7S8T9V0W", "session_index": 0 }] }
      ]
    },
    {
      "name": "open_items",
      "items": [
        { "text": "recap_test.go still skips the empty-store case",
          "cites": [{ "checkpoint_id": "01JQ8Z9K2M3N4P5Q6R7S8T9V0W", "session_index": 0 }] }
      ]
    },
    {
      "name": "dead_ends",
      "items": [
        { "text": "go test ./... -run TestRecap — failed 3x: undefined: recapFlags",
          "cites": [{ "checkpoint_id": "01JQ8Z9K2M3N4P5Q6R7S8T9V0W", "session_index": 0, "line": 412 }] }
      ]
    },
    { "name": "surface", "items": [], "note": "graph degenerate: no_dependents" },
    {
      "name": "stopped",
      "items": [
        { "text": "Stopped mid-edit in cmd/entire/cli/recap.go",
          "cites": [{ "checkpoint_id": "01JQ8Z9K2M3N4P5Q6R7S8T9V0W", "session_index": 0 }] }
      ]
    }
  ]
}
```

Go types (define these yourself in your package — do **not** import the `handoff` package):

```go
type Citation struct {
    CheckpointID string `json:"checkpoint_id"`
    SessionIndex int    `json:"session_index"`
    Line         int    `json:"line,omitempty"`
}
type Item    struct { Text string `json:"text"`; Cites []Citation `json:"cites"` }
type Section struct { Name string `json:"name"`; Items []Item `json:"items"`; Note string `json:"note,omitempty"` }
type Packet  struct {
    Repo        string    `json:"repo"`
    GeneratedAt time.Time `json:"generated_at"`
    Head        string    `json:"head_checkpoint_id"`
    Sections    []Section `json:"sections"`
}
```

**Two invariants you can rely on:** every `Item` has at least one `Citation`; a section that failed
has empty `Items` and a populated `Note` (it is never an error). Handle empty sections gracefully.

---

### 4.1 — Tier 1: ingest (MUST SHIP)

**Files you own:**

| File | What |
| --- | --- |
| `cmd/handoff-databricks/main.go` | flags, env config, stdin read, orchestration |
| `cmd/handoff-databricks/rows.go` | `Packet` → flat rows |
| `cmd/handoff-databricks/upload.go` | Files API upload + `COPY INTO` |
| `cmd/handoff-databricks/rows_test.go` | flattening table test |
| `cmd/handoff-databricks/upload_test.go` | `httptest.Server` round-trip |
| `cmd/handoff-databricks/testdata/packet.json` | the fixture |
| `databricks/ddl.sql` | table + volume DDL |

**Row flattening.** One packet becomes N rows, one per `(section, item, citation)`:

```go
type Row struct {
    RepoKey      string    `json:"repo_key"`
    CheckpointID string    `json:"checkpoint_id"`
    SessionID    int       `json:"session_id"`
    HeadID       string    `json:"head_checkpoint_id"`
    GeneratedAt  time.Time `json:"generated_at"`
    SectionName  string    `json:"section_name"`
    ItemIndex    int       `json:"item_index"`
    ItemText     string    `json:"item_text"`
    CiteLine     int       `json:"cite_line"`
    SectionNote  string    `json:"section_note"`
    IngestedAt   time.Time `json:"ingested_at"`
}
```

> **Key design — read this carefully.** The merge key is
> **`(repo_key, checkpoint_id, session_id)`** — *never `repo_key` alone*, which is **not unique**:
> repos without a remote get a synthetic `local/<basename>` key, so two different developers' local
> clones can collide on `local/cli`. For row-level uniqueness you additionally need
> `section_name` + `item_index`. State this in your DDL comment so a judge sees you knew.

`repo_key`: use the packet's `repo` field verbatim. It is already normalised upstream.

**Idempotency matters** — the same packet may be uploaded twice (re-runs, retries). Use `MERGE INTO`
on the full key, or `COPY INTO` (which is idempotent on already-ingested files by default).

**Transport: Files API + `COPY INTO`.** Deliberately chosen over the SQL Statement Execution API for
row insertion — it means **no SQL string building from packet text**, so there is no injection surface
and no escaping bugs on item text containing quotes or newlines.

```
PUT {host}/api/2.0/fs/files/{volume_path}/{repo_key}/{head_id}-{ts}.json
Authorization: Bearer {token}
Content-Type: application/octet-stream
<body: newline-delimited JSON rows>
```

Then one SQL statement to load it:

```
POST {host}/api/2.0/sql/statements
Authorization: Bearer {token}
{"statement": "COPY INTO handoff_packets FROM '<volume path>' FILEFORMAT = JSON ...",
 "warehouse_id": "<DATABRICKS_WAREHOUSE_ID>", "wait_timeout": "30s"}
```

The `COPY INTO` statement is a **constant** with no interpolated packet content — only the file path,
which you construct from sanitised identifiers. Keep it that way.

**`ddl.sql`** — Delta table with `repo_key`, `checkpoint_id`, `session_id`, `head_checkpoint_id`,
`generated_at`, `section_name`, `item_index`, `item_text`, `cite_line`, `section_note`, `ingested_at`.
Partition by `repo_key`. Add a comment naming the composite key and why.

### 4.2 — Tier 2: retrieval (the differentiator)

A **Vector Search** index over `item_text`, filtered to `section_name IN ('dead_ends','intent')`.
Delta Sync index is simplest — it tracks the table automatically.

Deliver a notebook `databricks/retrieval.py` exposing one query:

> given a task description, return the top-k prior dead ends with their `checkpoint_id` and
> `cite_line`, so the answer is **"three sessions ago someone tried this and it failed because X"**
> — *with a citation*, which is the whole product claim.

Keep the citation columns in the index payload. A retrieval result without a checkpoint ID is exactly
the unverifiable assertion this project exists to eliminate.

### 4.3 — The offline fallback (REQUIRED, not optional)

If `DATABRICKS_HOST` starts with `file://`, write the NDJSON rows to that local directory instead of
calling any API, and exit 0.

This is **required**, because the 12:00 curveball demo must not hard-depend on a workspace, a network,
or a token that might be rate-limited. Build it in Tier 1, not as an afterthought. It is also what
makes your `rows.go` testable without any HTTP at all.

### 4.4 — Tier 3: MLflow evaluation (if time)

Hand-label ~20 `(task description → expected dead end)` pairs. Run an MLflow eval over the retrieval
from 4.2 and report recall@k. This turns *"our retrieval is relevant"* from a claim into **a number in
the submission**, which is worth disproportionately more than another feature.

---

## §5 — Also assigned to you: graph evidence #1 and #2

Two of three mandatory graph artefacts (**15 scored points**, and they are process artefacts costing
minutes, not hours). You have genuine slack while the contract is being written, so they are yours.

```bash
entire plugin install graph
entire graph version
entire graph init-agents --repo .
# then START A FRESH AGENT SESSION so it picks up the graph instructions
```

- **Artefact #1 — a definition lookup.** e.g.
  `entire graph def --repo . --symbol runHandoff --format json`
- **Artefact #2 — an impact analysis *before* a high-risk change.** Must be captured *before* the
  change lands, so do it early.

Save raw JSON output to a scratch file and tell the integrator where it is. Note that
`entire graph impact` **requires `--symbol`** — it will not accept a bare file path.

**Disclosure to hand to the integrator:** `dependents_count` is a heuristic textual identifier index,
not a true graph query; it can emit `W_ANALYSIS_BUDGET_EXCEEDED` and undercount. We disclose this in
`BUILDATHON.md` ourselves — a judge finds it in ninety seconds, and naming it first scores better than
hoping.

---

## §6 — Branch and hand-off

- Branch: **`ws/databricks`**. Push there; **never push to `main`** — the integrator merges with
  `--no-ff`.
- Rebase before asking for a merge: `git fetch && git rebase origin/main`. It should be a clean no-op
  every time. **If it is not, you have touched someone else's file — stop and say so.**
- Commit small and often with real messages. Checkpoint quality is scored on whether checkpoints
  "preserve useful intent and decision context" — a commit message saying `wip` wastes a scored
  artefact. Record decisions, rejected options, and assumptions.

## §7 — Definition of done

- [ ] `go test ./cmd/handoff-databricks/...` green, tests `t.Parallel()` where env allows
- [ ] `./handoff-databricks < testdata/packet.json` with `DATABRICKS_HOST=file:///tmp/out` writes NDJSON
- [ ] Against a real workspace: rows land in the Delta table, keyed `(repo_key, checkpoint_id, session_id)`
- [ ] Re-running the same packet does **not** duplicate rows (idempotent)
- [ ] `databricks/ddl.sql` committed, with the composite-key rationale in a comment
- [ ] Vector Search returns prior dead ends **with citations** (Tier 2)
- [ ] `mise run fmt && mise run lint` clean
- [ ] No credential in any tracked file, in markdown, or on screen in the recording
- [ ] Graph artefacts #1 and #2 captured and their location reported
