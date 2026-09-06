# entire handoff

## One-sentence summary

`entire handoff` turns a repository's Entire Checkpoints into a short, fully
cited briefing — intent, open items, dead ends, blast radius, where work
stopped — so the next coding session starts oriented instead of blind, and can
trace every line back to the checkpoint it came from.

## Problem, intended user and why it matters

Every new AI coding session starts with no memory of the last one. It re-tries
an approach that already failed, re-asks a question that was answered
yesterday, and quietly drops the three things a previous session promised to
come back to. The only fix available today is a human re-explaining the
situation from memory — and people are worst at remembering exactly the thing
that matters most: which of five approaches was the one that did not work.

The intended user is whoever picks the work back up: the same developer the
next morning, a teammate on a different machine, or a fresh agent session with
an empty context window. Git tells them what changed. It cannot tell them what
was attempted and abandoned, or why.

This matters because the failure is silent and repeated. A dead end that is not
recorded is not just lost — it is re-paid for, in full, by the next session.

## Selected Entire track and why Entire is essential

**Track 1 — Build a Checkpoint-Native Developer Experience.**

Entire is not instrumentation here; it is the only source of the data. The
product's three most valuable outputs cannot be produced from a git diff at all:

- **Dead ends** come from `result.status` blocks inside the checkpoint's
  `transcript.jsonl`, written by Entire's own `transcript/compact` pipeline.
  No git artifact records that a tool call failed, let alone what the agent
  said immediately afterward about why it was giving up on that approach.
- **Intent and open items** come from checkpoint `Summary` fields, which exist
  because Entire captured the session, not because someone wrote a good commit
  message.
- **Citations** — the property that makes the packet trustworthy — are
  checkpoint IDs. Every line resolves under `entire checkpoint explain`, so a
  reader can open the session behind any claim. A briefing you cannot verify is
  a rumour with better formatting.

The blast-radius section additionally consumes the Entire Graph plugin
(`entire graph diff`) for entity-level change impact.

## Architecture and main workflow

```
Entire checkpoint store  ──▶  handoff.Load  ──▶  Input (newest-first)
                                                   │
              ┌──────────┬──────────┬──────────┬───┴──────┬───────────┐
              ▼          ▼          ▼          ▼          ▼           ▼
           intent   open_items  dead_ends   surface    stopped    prior_art
                                                (graph)          (opt-in only)
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

Three properties hold across every section:

1. **Every item carries a citation.** Enforced mechanically by
   `TestEveryItemIsCited` across all extractors — an uncited item is a bug, not
   a style preference.
2. **Failure is a Note, not an error.** A section that cannot do its job
   returns no items and a stated reason. A packet that refuses to print because
   the graph was down is worse than useless to whoever needs orienting.
3. **Extractors never touch the store.** Everything is loaded once into
   `Input`, so each section is a pure function over a struct — which is why the
   tests run in milliseconds with no git repository.

Main workflow: `entire handoff` (markdown) or `entire handoff --json` (stable
machine shape). A fresh agent bootstraps itself via the managed skill installed
by `entire enable --handoff-skill`, whose single instruction is to run the
command *before reading any file*.

## Entire Graph findings and verification

Four artifacts, captured against `entire-graph` v0.4.0 and committed:

| Artifact | Command | What it showed |
| --- | --- | --- |
| `graph-1-def.json` | `graph def --symbol runHandoff` | Definition lookup: `runHandoff` at `cmd/entire/cli/handoff.go:71-114`, one declaration, no ambiguity. |
| `graph-2-impact.json` | `graph impact --symbol Load` | **`Load` is ambiguous across 10 symbols** in this repo (`handoff.Load`, `settings.Load`, `session.StateStore.Load`, `redactCache.load`, …), and the graph said so via `disambiguation_required` rather than guessing. That is itself the finding: a symbol-name-based impact query on a common verb needs qualifying. |
| `graph-3-diff.json` | `graph diff --base dd7eaa8c6 --head HEAD` | Final semantic diff across every commit that built this feature. |
| `graph-4-curveball-impact.json` | `graph impact --symbol NewVectorSearcher` | The Curveball's affected area: the external-egress path, confirmed to have exactly one construction site and one call path, so the redaction fix had one correct place to live. |

**One honesty note on artifact 4.** The Curveball asks for impact analysis
*before* changing the affected area. The query was run before the edit, but
that run died on an invalid `--depth 3` (the plugin accepts 1 or 2) and saved
nothing, so the artifact committed here is the successful re-run, taken at
commit `3a4fec2` — i.e. after the change, not before. The finding is identical
either way and checkable against source in about a minute, but the artifact is
not the pre-change snapshot the card asked for, and calling it one would be
exactly the sort of unverifiable claim this product exists to prevent.

**Verification, stated honestly.** The graph's own output reports
`"completeness_level": "degraded"` on this repo: `databricks/ddl.sql` fails
tree-sitter parsing (`E_PARSE_ERROR`) and `.entire/runners/trail-security.json`
is skipped as minified (`E_MINIFIED`), and every run carries
`W_WORKTREE_SNAPSHOT`. We read those warnings rather than the headline numbers,
and we verified the findings that mattered against source directly — the
privacy-boundary fix was placed by reading `globalmemory.go`, with the graph
used to confirm no *second* egress path existed, not to locate the first.

Correction worth recording: the original plan called for running `graph impact`
per touched file. That cannot work — `impact` requires `--symbol`, and a
checkpoint carries `FilesTouched`, which are paths. The `surface` section and
artifact 3 both use `graph diff` instead, which needs no symbol input.

## Noon Curveball: what changed and how we adapted

**The constraint (Track 1 — Privacy Boundary):** the product must work on
sensitive repositories. Raw prompts and transcripts must not reach a new
external service; output must stay useful when fields are redacted or missing;
existing local behaviour must keep working; the interface must distinguish
complete from incomplete context and must never present incomplete context as
authoritative; and at least one test must use redacted or missing checkpoint
data. The card is committed verbatim as [`curveball.md`](curveball.md).

**What it exposed.** The feature added immediately before noon — cross-repo
retrieval (`--ask`) — was the violation. It POSTed query text to Databricks
Vector Search, and that text was either the user's `--ask` string typed
verbatim or, when empty, a checkpoint's `Summary.Intent`. Both are
prompt-shaped, and neither had been through redaction at that point.

**What changed:**

1. **Egress is redacted** (`handoff/globalmemory.go`). `redact.String` runs over
   the query immediately before it enters the HTTP body — the same call
   `checkpoint/persistent.go:1219` already makes on `Summary.Intent`, and one
   that uses only local scanners, so closing a network exposure does not open
   another.
2. **The ingest half needed no change, and we checked rather than assumed.**
   `cmd/handoff-databricks` never imports the handoff package and never sees
   `Checkpoint.Transcript`; it consumes packet JSON on stdin, so the only text
   it can send is `Item.Text` — already extractor-derived, already truncated,
   already redacted at rest by the checkpoint pipeline.
3. **Incomplete context became visible instead of implied.** `Section` gained an
   `Incomplete` flag, set centrally in `BuildWith` from `Input.hasGaps()`
   (unreadable or truncated checkpoints, or any loaded checkpoint with no
   Summary or no Transcript). It is set *after* every extractor returns, so a
   future section cannot forget to set it — the failure mode a per-extractor
   flag would have had. Markdown gained a banner and a per-heading
   `(incomplete)` marker; `--json` carries the flag.

**Tests** (`handoff/privacy_test.go`): a redacted-checkpoint fixture — Summary
and Transcript absent, Metadata intact — still produces cited, usable output
*and* flags itself; the inverse case, so a complete range is never mislabelled
(otherwise the warning trains readers to ignore it); and both query paths,
asserting a planted credential never reaches the outgoing text. No fixture file
arrived with the card, so the fixture is constructed from its wording, in the
test itself.

**A finding we did not expect, and are reporting rather than burying.** Those
tests were first written with an AWS access key id (`AKIA…`) as the planted
secret, and they *failed*: `redact.String` passes bare AWS key ids through
untouched — including AWS's own documented `AKIAIOSFODNN7EXAMPLE`, and
including one sitting immediately after an `aws_access_key_id =` prefix. Its
entropy (3.88) is below the 4.5 threshold for the entropy layer, and the
betterleaks ruleset does not claim that shape in isolation. That is a gap in
the shared redaction pipeline, not in this feature's boundary. The fixture was
changed to a GitHub PAT shape, which the pipeline does catch, and
`TestPrivacyBoundary_FixtureSecretIsActuallyRedactable` now pins that
assumption so a ruleset change cannot silently make the other two tests
vacuous. The underlying `redact` gap is worth a separate issue upstream.

**What we did not do:** we did not remove the retrieval feature. Deleting it
would have satisfied the letter of the constraint and lost the product's
cross-repo claim. Redacting at the boundary keeps the capability and makes the
boundary explicit.

## Checkpoint links and what each checkpoint proves

Repository: `https://github.com/NiharikaPanda09/cli` · branch `main`.
Open any of these with `entire checkpoint explain <id>`.

| Milestone | Checkpoint | Commit | What it proves |
| --- | --- | --- | --- |
| Initial understanding and intended architecture | `01M1TS1K9VQC4BW89ASRD24PBE` | `dee5103` | Plan-vs-reality accounting: what was built, what was cut, and three factual errors found in our own planning documents. |
| (supporting) | `01M1TRGYGTEBC9AXARF0P0Y156` | `dbd13de` | The labelled sample packet — the output shape the whole product is arguing for. |
| Last stable state before the Curveball | `01M1TT6RE9KGDW6DN85X2WY26G` | `426cc65` | Cross-repo retrieval, `--ask`, `--session` — the feature the Curveball then forced us to re-examine. |
| (supporting) | `01M1TX105EAMH54875CXRSRRYQ` | `7d48361` | Pre-curveball stable state: graph evidence captured, submission writeup started. |
| **Response to the Noon Curveball** | `01M1TZ8MG0RKM14JG48SCRJTKC` | `3a4fec2` | The privacy boundary: redacted egress, `Incomplete` flag, and the redacted-checkpoint tests. |
| Final implementation and verification | *this commit* | — | Final `BUILDATHON.md`, graph artifacts, verification status. |

**Disclosed:** the first six commits of this feature carry **no** checkpoint.
Entire's hooks were installed the whole time, but every invocation silently
no-op'd because the `entire` binary was not on `$PATH`. Capture resumed on the
very next turn after the fix, with no session restart. Those six commits cannot
be reconstructed, and we would rather say so than quietly present five
checkpoints as if they were the whole history.

## Setup, run and test instructions

```bash
# Build (Go 1.26+; no CGO, no tree-sitter dependency)
go build -o ~/.local/bin/entire ./cmd/entire
export PATH="$HOME/.local/bin:$PATH"     # hooks resolve `entire` via PATH

# Run — reads only; writes nothing
entire handoff                    # markdown briefing
entire handoff --json             # stable machine-readable packet
entire handoff --limit 20         # reach further back
entire handoff --no-graph         # skip the blast-radius section

# Follow any citation back to its session
entire checkpoint explain <checkpoint_id>

# Let a fresh agent bootstrap itself
entire enable --handoff-skill --agent claude-code

# Test
go test ./cmd/entire/cli/handoff/... ./cmd/handoff-databricks/...
go test -run TestPrivacyBoundary ./cmd/entire/cli/handoff/...   # curveball behaviour
```

**Verification status.** Observed on Go 1.26.8, after the Curveball changes:
`gofmt -l cmd/entire/cli/handoff/` clean, `go build ./cmd/entire/...` clean,
`go test ./cmd/entire/cli/handoff/...` → **ok, 0.572s**. The pre-Curveball tree
was separately verified green including `golangci-lint` (recorded in commit
`426cc65`).

## Databricks use, data sources and limitations

**We are not claiming the Best Use of Databricks award.** The lane is built but
not live, and that distinction matters more than the points:

| Tier | Status |
| --- | --- |
| Delta ingest — `handoff-databricks`, Files API + `COPY INTO`, keyed `(repo_key, checkpoint_id, session_id)` | Built, tested |
| `file://` offline fallback (demo needs no workspace) | Built, tested |
| Vector Search retrieval + `--ask` | Code complete; tested against an `httptest` server, **never against a live index** |
| A provisioned endpoint or index | **Never created.** No workspace exists |
| MLflow evaluation | Not built |

Transport was chosen so no packet text ever enters a SQL string. Credentials
come from the environment only, never from a settings file — those are
version-controlled, so a token in one ships to everyone who clones. Data
provenance: rows derive from this repo's own checkpoints, which are redacted at
rest by Entire's pipeline before the packet ever sees them, and the Curveball
commit added redaction on the outbound query path too.

Offline demo, no workspace or token required:

```bash
DATABRICKS_HOST=file:///tmp/out entire handoff --json | handoff-databricks
```

## Known limitations and next steps

Stated here rather than left to be discovered:

- **Dead-end mining is Claude Code only.** `transcript/compact.Compact` sniffs
  OpenCode, Gemini, pi, Codex, Copilot and Droid before the Claude path, and
  several agents produce no usable `transcript.jsonl` at all. The other four
  sections work for any agent.
- **Grouping is a heuristic, not semantic.** Failures group by exact command,
  then path, then tool name. Two commands that mean the same thing spelled
  differently become two entries.
- **`dependents_count` can undercount.** It is a heuristic textual identifier
  index, not a graph query; it emits `W_ANALYSIS_BUDGET_EXCEEDED` and degrades
  silently under budget pressure. `entire-graph`'s own help text calls it "a
  heuristic dependent count." Degenerate entities are suppressed rather than
  dressed up.
- **The `surface` section is untested against live graph output** in its unit
  tests — covered by an injected fake, so the real field names were an informed
  guess until the artifacts above were captured. A mismatch degrades to the
  touched-file list rather than breaking.
- **No fork-and-mirror step was performed.** Work happened in a direct clone of
  the fork rather than through `entire repo mirror create` + `entire repo
  clone`, so there is no mirror URL to submit. This is a process divergence, not
  a product one, and it is ours to own.
- **No demo recording exists.**
- **The `handoff` command is registered unlisted and read-only**
  (`"handoff": {agentHelpAudienceReadOnly, false}`). Read-only is factual — it
  opens the store and prints. Unlisted is this repo's documented safe default
  for a new command; promoting it into the advertised agent surface is a
  product decision for whoever owns that surface.

**Next steps, in order:** run the test suite on a Go-equipped machine; provision
a Vector Search index and replace the fake-server retrieval test with a live
one; extend dead-end mining to a second agent's transcript format; and add the
MLflow evaluation that would turn "our retrieval is relevant" from a claim into
recall@k.
