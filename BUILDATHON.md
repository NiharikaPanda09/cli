# `entire handoff` — Buildathon submission (Track E1)

A cited handoff packet for AI coding sessions: `entire handoff` reads a repo's
Entire checkpoints and prints what the last sessions were trying to do, what is
still open, what was already tried and failed, what area of the code is at
risk, and where work stopped — with every line traceable to the checkpoint it
came from.

---

## 1. The problem and the intended user

Every new AI coding session starts with no memory of the last one. It re-tries
an approach that already failed, re-asks a question that was already answered,
and quietly drops a promise a previous session made to come back to something.
Today the only fix is a person re-explaining the situation from memory, and
people are bad at remembering which of five approaches was the one that didn't
work.

The intended user is anyone resuming work an agent (or a teammate using an
agent) left off — the next morning, on a different machine, or as a different
person entirely. Entire already records every session as it happens; nobody
reads the raw recording because it's enormous. `entire handoff` is the thing
that reads it for you and prints a short, verifiable briefing instead.

## 2. Demand citations

`cli#408`, `#1125`, `#381`, `#985`, `#296`. Of these, `#1125` is the most
specific: it asked for a machine-readable handoff packet by name, which is
exactly what `entire handoff --json` is. `#381` was closed pointing at the
shipped `session-handoff` skill as a sufficient answer — §3 below is our
response to that.

## 3. Positioning against the shipped `session-handoff` skill

`session-handoff` is a real, shipped official skill
(`cmd/entire/cli/telemetry/skill_official.go:56`) described as inspecting
recent sessions or summarizing a saved one
(`agent/skilldiscovery/match_test.go:32`). It is a reasonable objection to this
whole project, so we answer it directly rather than assert novelty.

Three things `entire handoff` does that it does not:

1. **Dead-end mining from `result.status` blocks in the transcript.** No CLI
   command or exported helper surfaces which tool calls actually failed today.
   `entire handoff` parses `transcript.jsonl`, groups repeated failures by
   command/path/tool, and keeps the assistant text that follows an errored
   call — the sentence explaining *why* the approach was abandoned. This is
   the differentiator: "three sessions ago, `go test -run TestRecap` failed
   twice with `undefined: recapFlags`" is not a sentence the existing skill
   can produce.
2. **Graph-scoped blast radius.** The `surface` section shells out to `entire
   graph diff` over the range of touched checkpoints and reports entity-level
   changes with dependent counts — which parts of the codebase are now at risk
   because of what the last sessions touched.
3. **A stable, documented, machine-readable packet** — `entire handoff --json`
   — which is what `cli#1125` asked for by name. Its shape (five named
   sections, `Item`/`Citation` pairs) is a published interface other tools can
   consume; it is what feeds both the managed skill and the Databricks lane
   below.

## 4. The curveball

None was issued. We are saying so plainly rather than inventing one:
`IMPLEMENTATION_ROADMAP.md` was written expecting a 12:00 curveball card, and
`docs-private/STATUS_AND_DIVERGENCES.md` (written after the fact, from the
actual tree) records that step 8 of the plan — "noon curveball protocol" — is
not applicable because no curveball ever arrived.

## 5. Three disclosures

Stated in our own words, because a judge who opens the relevant file finds
each of these in under two minutes, and we'd rather be first to say them:

- **Dead-end mining is Claude Code only.** `transcript/compact.Compact` sniffs
  OpenCode, Gemini, pi, Codex, Copilot, and Droid before the Claude/Cursor
  path, and `external-agents#67` reports that amp/goose/kilo/kiro/qwen produce
  no usable `transcript.jsonl` at all. Every other section (intent, open
  items, stopped) reads session summaries and works for any agent; only
  dead-end mining is Claude-Code-specific.
- **Dead-end grouping is a heuristic, not semantic matching.** Failures group
  by exact `command`, then file `path`, then tool name. Two commands that mean
  the same thing but are spelled differently land in separate entries. There
  is no fuzzy or semantic grouping anywhere in the packet.
- **`dependents_count` in the surface section is a heuristic textual index,
  not a graph query**, and it can undercount. It comes from `entire graph
  diff`'s dependent-count analysis, which emits `W_ANALYSIS_BUDGET_EXCEEDED`
  and degrades silently under budget pressure — the tool's own help text
  (`entire-graph/internal/cli/help.go:319`) calls it "a heuristic dependent
  count." We suppress degenerate entities rather than dressing them up, and
  the section's own output says so via its `Note`.

## 6. The `handoff` classification call

Registered as **unlisted, read-only** (`agent_help_cmd.go`:
`"handoff": {agentHelpAudienceReadOnly, false}`). Read-only is not a judgment
call — the command only opens the checkpoint store and prints; it writes
nothing to the repo or the working tree. Unlisted is the safe default per this
repo's own convention (`CLAUDE.md`: "Take the safe default (unlisted,
user-owned)... so a human can move it"): promoting a brand-new command into
the advertised `agent-help` listing is a product decision for someone who owns
that surface, not something to decide unilaterally while adding the command.

## 7. Links

- Sample labelled packet (for demoing shape without a real repo history):
  [`cmd/entire/cli/handoff/testdata/sample_packet.json`](cmd/entire/cli/handoff/testdata/sample_packet.json)
- Architecture and data contract: [`docs/architecture/handoff.md`](docs/architecture/handoff.md)
- Divergences from the original plan, with reasons:
  [`docs-private/STATUS_AND_DIVERGENCES.md`](docs-private/STATUS_AND_DIVERGENCES.md)
- Graph evidence artifacts (see §9 below): `graph-1-def.json`, `graph-2-impact.json`,
  `graph-3-diff.json`
- No recording exists. The curveball demo this would have shown didn't happen
  because no curveball was issued (§4).

---

## 8. Checkpoints (scored artefact)

Four real checkpoints exist, pushed to `origin` and independently resolvable
via `entire checkpoint explain`, verified at the time of writing this file:

| Checkpoint | Commit | What it covers |
| --- | --- | --- |
| `01M1TRGYGTEBC9AXARF0P0Y156` | `dbd13de` | Labelled sample packet for demoing handoff output |
| `01M1TS1K9VQC4BW89ASRD24PBE` | `dee5103` | Build status, divergences from the plan, and a correction |
| `01M1TT6RE9KGDW6DN85X2WY26G` | `426cc65` | Cross-repo retrieval, `--ask`, and `--session` |
| `01M1TVNFQ53G98WWV17TE10EXB` | `2b95cfd` | Handover notes for continuing on another machine |

Six earlier commits on this feature have no checkpoint behind them: Entire's
hooks were installed throughout, but every invocation silently no-op'd because
the `entire` binary was not on `$PATH` (it only existed at a throwaway build
location). This is disclosed rather than hidden — see
`docs-private/STATUS_AND_DIVERGENCES.md` §5.1 for the full account, including a
correction of an earlier, wrong version of that same story.

## 9. Graph evidence (scored artefact)

Three artifacts, captured against `entire-graph` v0.4.0:

1. **Definition lookup** — `entire graph def --repo . --symbol runHandoff --format json`
   → `graph-1-def.json`
2. **Impact analysis** — `entire graph impact --repo . --symbol Load --format json`
   → `graph-2-impact.json`. `Load` (`handoff/input.go`) is the function every
   extractor's data ultimately passes through, so this is the impact query
   that matters most for this feature.
3. **Final semantic diff of the submitted implementation** —
   `entire graph diff --base dd7eaa8c6 --head HEAD --json` → `graph-3-diff.json`,
   covering every commit that built this feature (`dd7eaa8c6` is the commit
   immediately before the first handoff commit).

Note on the plan's own claim: the original design called for running `entire
graph impact` per touched file. That does not work — `impact` requires
`--symbol`, and a checkpoint's `FilesTouched` are paths, not symbols. This is
why the `surface` section (item 2 in §3) and artifact 3 above both use `entire
graph diff` instead, which needs no symbol input and returns entity-level
changes directly.

## 10. What is honestly not built

Said plainly rather than left for a judge to discover:

- **No live Databricks Vector Search index has ever been created.** The
  ingest path (`handoff-databricks`) and the retrieval path (`--ask`, the
  opt-in `global_memory` section) are both implemented and tested against a
  fake HTTP server, but neither has run against a real workspace. `ddl.sql`
  documents the REST calls to create the endpoint and index because they are
  not SQL.
- **MLflow evaluation was not built.** It would turn "our retrieval is
  relevant" from a claim into a number (recall@k over ~20 hand-labelled
  `task → expected dead end` pairs); it remains a good next step, not a
  finished one.
- **No MCP tool.** `grep entire_handoff cmd/entire/cli/mcp.go` returns
  nothing. This was optional per the plan and skills were always the intended
  primary surface (`cli#292` was closed `wontfix` in favor of skills for the
  same reason).
- **The `surface` (blast-radius) section is untested against live `entire
  graph diff` output** in its unit tests — it is covered by an injected fake,
  so the field names it expects from a real run were an informed guess before
  today. Capturing the graph artifacts above for this document is the first
  time it has been exercised against the real plugin.

Also worth stating because it appeared, wrongly, in an earlier internal
write-up: this build does not need CGO (there is no tree-sitter dependency in
`go.mod`), does not use a Databricks Go SDK (`grep databricks go.mod` → no
match; retrieval and ingest are plain `net/http`), and extractors do not
mutate a shared packet — each returns its own `Section` and a separate
assembly step in `build.go` collects them.
