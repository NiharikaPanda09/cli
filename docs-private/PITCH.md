# Pitch: `entire handoff`

Everything below is checkable against the tree. Nothing here is aspirational —
where something is not built, it says so.

---

## The problem

Every AI coding session starts from zero.

The agent has no memory of yesterday. So it proposes an approach that was tried
and abandoned last week, re-asks a question answered on Tuesday, and quietly
drops the three things it promised to come back to. The only fix available today
is a human re-explaining the situation by hand, from memory — and humans are
worst at remembering exactly the thing that matters most: **which of five
approaches was the one that didn't work.**

That failure is invisible and expensive. Nobody logs "we wasted forty minutes
re-deriving that this API doesn't support batching." It just happens again.

**The insight:** the record already exists. Entire captures every session as it
happens. Nobody reads it, because it is enormous and unreadable. The knowledge
is not missing — it is unretrievable.

---

## Before vs now

| | Before | Now |
| --- | --- | --- |
| Getting context to a new session | Human pastes it from memory | `entire handoff` — one command |
| Knowing what already failed | **Nothing surfaces it.** No command, no exported helper | `dead_ends`, mined from `result.status` in the transcript |
| Trusting what you're told | A summary you cannot check | Every line carries a checkpoint id; `entire checkpoint explain <id>` opens the session |
| Feeding an agent | Prose written for humans | Stable JSON built for machines |
| Scope of memory | Your machine, your session | Rows in a Delta table the team shares |

The last row is the smallest today and the largest tomorrow — see the honesty
section.

---

## What we built

**Five sections**, each one cited: what we were trying to do, what's still open,
**what already failed**, what got touched, where work stopped.

**The dead-end miner is the product.** It reads `result.status` out of the raw
transcript — data **no CLI command and no exported helper surfaces today**. The
obvious shortcut, `compact.BuildCondensedEntries`, cannot do it: it builds tool
entries from `name` and `input` only and discards `result` before any caller
sees it. So we wrote our own parser.

**A managed skill** so a fresh agent runs `entire handoff --json` before reading
any file. No pasting.

**A warehouse lane** — a separate binary shipping packet rows into Delta, keyed
`(repo_key, checkpoint_id, session_id)`.

3,527 lines across 30 files. **56 test functions.** `go build`, `go vet`,
`gofmt`, `golangci-lint` all clean. **No existing behaviour changed** — every
pre-existing component is read-only from our side.

---

## Why it holds up

**Every claim is traceable.** This is the whole design, not a feature. A test,
`TestEveryItemIsCited`, enforces it mechanically across every extractor — an
uncited item fails the build. A retrieved row that comes back without a
checkpoint id is dropped rather than shown.

**It degrades instead of dying.** A section that cannot do its job returns a
reason, not an error. Graph down, no summaries, unreadable checkpoints — the
packet still prints. A packet that refuses to render because one section failed
is worse than useless to the person who needs orienting.

**It survives contact with a real repo.** Two bugs only showed up against real
data, both now fixed: 100 checkpoints reported as "no checkpoints in range", and
an unbounded walk that hung for **over ten minutes** (now 13 seconds, with an
honest note).

**The miner is tested against the real writer.** Not just our own fixture —
`transcript_realdata_test.go` runs against
`transcript/compact/testdata/claude_expected2.jsonl`, the compact writer's own
committed expected output, which holds 4 real `status:"error"` blocks. If the
writer format ever drifts from our parser, that test fails.

---

## Counter-questions, and honest answers

**"Isn't this just the `session-handoff` skill you already ship?"**
The strongest question, and it has a concrete answer. That skill summarises
sessions. It does not mine `status:"error"` blocks, does not give you graph-scoped
blast radius, and does not emit a stable machine-readable packet — the thing
`cli#1125` asked for by name. Sections 1, 2 and 5 are admittedly close to what a
summariser gives you. **Section 3 is not, and cannot be**, because the data it
reads is discarded before any existing helper can reach it.

**"So it's a summariser with extra steps?"**
A summariser tells you what happened. This tells you **what to not do again**,
with a line number. Different question, different answer.

**"Your dead-end grouping is naive."**
Correct. It groups on exact command, then file path, then tool name. Two
semantically identical commands written differently land in separate entries. We
are not claiming otherwise — it is stated in the limits and in the output.

**"Why is there no semantic search?"**
The retrieval code is written and tested against a fake server. **It has never
run against a live index.** No index has been created; `ddl.sql` builds a view
shaped for one. We would rather say that than demo something we have not seen
work.

**"You're using the Databricks SDK, right?"**
No. `grep databricks go.mod` returns **0**. We use a Delta Sync index with
managed embeddings, so Databricks embeds both the stored column and the query,
and the CLI just POSTs a query string over `net/http`. That was a deliberate
choice to avoid pulling a dependency tree in for work the REST API already does.

**"Does this leak secrets into a warehouse?"**
Tool output is truncated to ~200 characters, which is a mitigation and not a
guarantee — a short key fits. The real protection is upstream: Entire's existing
redaction pass runs before anything is stored, so the packet only ever reads
already-scrubbed text. We did not weaken that; we inherit it.

**"It only works with Claude Code."**
For dead ends, yes. Other agents do not reliably produce a usable
`transcript.jsonl`. The other four sections work anywhere summaries exist.

**"`dependents_count` is a real graph query?"**
No — it is a heuristic textual identifier index that can undercount and emits
`W_ANALYSIS_BUDGET_EXCEEDED` under budget pressure. Our own section note says so
in its output. A judge who opens `entire-graph` finds this in ninety seconds;
better that we say it first.

**"How do I know the packet isn't hallucinated?"**
Take any line, copy its checkpoint id, run `entire checkpoint explain <id>`. It
opens the actual session. That is the entire point — and it is why we refused to
seed fake checkpoints for the demo, even under time pressure. A demo that dies
under `checkpoint explain` is worse than no demo.

**"Show me it working on real data."**
The miner, yes — against the compact writer's own fixture. End-to-end on this
repo's own checkpoints is thin, because Entire's hooks could not find the CLI on
PATH for most of this build, so only the last few commits were captured. That
was our environment mistake, and it is written up rather than hidden.

---

## What we do not claim

Not built: **MLflow evaluation**, the **MCP tool**, and **any live Vector Search
index**. The retrieval path exists in code and passes tests against a fake
server; it has never returned a real row.

Do not describe the warehouse lane as "search" or "semantic memory". Today it is
ingest plus a view. The retrieval code is ready for the day an index exists.

---

## The one-sentence version

**Entire already records every AI coding session; we make the failures in those
recordings retrievable, so the next session doesn't repeat them — and every line
we print can be traced back to the moment it happened.**
