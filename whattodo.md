# What to do next

Handover notes for continuing `entire handoff` on another machine.
Written 2026-09-06 from the macOS box. Everything below is verified, not assumed.

---

## 0. Get the code first — it is NOT pushed

**5 commits exist only on the macOS machine.** Do not start work on Linux until
these are on the remote, or you will duplicate or lose them.

```
b35246429  Add cross-repo retrieval, --ask and --session
20b1d9c59  Record build status, divergences from the plan, and a correction
aaf097182  Add a labelled sample packet for demoing handoff output
b2684fc1a  Verify Entire hooks now capture commits
46fea969e  Bound handoff checkpoint reads and report unreadable ones
```

**Why the push fails — it is a permissions mismatch, not a network problem:**

```
remote: Permission to NiharikaPanda09/cli.git denied to Niharika-Shipthis
```

The repo is owned by **NiharikaPanda09**, but git is authenticating as
**Niharika-Shipthis** (`niharika@shipthis.co`). Two different GitHub accounts.
`git ls-remote` succeeds because reading is allowed; writing is not.

Pick one:

- **Log in as the repo owner** — `gh auth login` as `NiharikaPanda09`, then push.
- **Give `Niharika-Shipthis` write access** to `NiharikaPanda09/cli` on GitHub.
- **Push to a fork you own** — `git remote add mine <url> && git push mine main`.

Nothing else in this list matters until the commits are somewhere you can clone.

---

## 1. Set up the Linux machine

The macOS box needed manual setup and Linux will too. **Neither Go nor mise was
installed here** — I installed Go via brew and put the binary on PATH by hand.

```bash
# Go 1.26.6+ is required (go.mod). Go 1.27.1 was used on macOS and works.
go version

# Build and install the CLI somewhere on PATH.
go build -o ~/.local/bin/entire ./cmd/entire
export PATH="$HOME/.local/bin:$PATH"        # add to ~/.bashrc to persist
entire version
```

**This step is not optional and is the single biggest trap.** Entire's git hooks
run `command -v entire`. If the binary is not on PATH, every hook silently
prints *"Entire CLI is enabled but not installed or not on PATH"* and exits —
which is exactly why the first six commits of this project have **no checkpoints
at all**. Verify before doing anything else:

```bash
sh -c 'command -v entire >/dev/null && echo HOOKS-WILL-RUN || echo BROKEN'
```

Then confirm a commit actually records:

```bash
git commit --allow-empty -m "check capture"
git log -1 --format=%B | grep -c "Entire-Checkpoint:"   # must print 1
```

Optional but useful:

```bash
entire plugin install graph      # already installed on macOS, v0.4.0
golangci-lint --version          # needed before pushing; CI enforces it
```

---

## 2. Capture the three graph artefacts — 15 points, minutes of work

**None have been captured.** The graph plugin is installed (v0.4.0) but was
never run. This is the highest value-per-effort item left.

```bash
entire graph init-agents --repo .
# then START A FRESH AGENT SESSION so it picks up the graph instructions

# Artefact 1 — a definition lookup
entire graph def --repo . --symbol runHandoff --format json > graph-1-def.json

# Artefact 2 — impact analysis BEFORE a high-risk change (do it before editing)
entire graph impact --repo . --symbol Load --format json > graph-2-impact.json

# Artefact 3 — semantic diff of the final implementation (do this last)
entire graph diff --repo . --format json > graph-3-diff.json
```

`entire graph impact` **requires `--symbol`** and will not accept a bare file
path. That is why `surface.go` uses `graph diff` instead.

---

## 3. Write `BUILDATHON.md` — required for submission

Does not exist. Use these headings, in this order:

1. **The problem and the intended user.**
2. **Demand citations:** `cli#408`, `#1125`, `#381`, `#985`, `#296`.
3. **Positioning against the shipped `session-handoff` skill.** Not optional —
   `cli#381` was closed pointing at it, and whoever wrote that reply may be
   judging. Name the three things it cannot do: dead-end mining from
   `status:"error"` transcript blocks, graph-scoped blast radius, and the
   machine-readable packet `#1125` asked for by name.
4. **The curveball** — none was issued; say so plainly rather than inventing one.
5. **The three disclosures**, stated in our own words first:
   - Dead ends are **Claude Code only** — other agents produce no usable `transcript.jsonl`.
   - Dead-end grouping is a **heuristic** (exact command / path / tool name).
   - `dependents_count` is a **textual index that can undercount**, emitting
     `W_ANALYSIS_BUDGET_EXCEEDED`.
6. **The `handoff` classification call** — unlisted, read-only, and why.
7. **Links** to the sample packet and any recording.

Full source material is in `docs-private/STATUS_AND_DIVERGENCES.md` — it already
has the divergences, the bugs found, and the honest positioning paragraph.

**Do not claim** vector search, semantic memory, embeddings, MLflow, or "the
whole organisation's history" as working features. See §5.

---

## 4. Finish the Databricks lane (needs a workspace)

The Go side is **done and tested**; only workspace provisioning is left.

```bash
# 1. Apply the DDL — it enables Change Data Feed and adds the row_id key.
#    A Delta Sync index fails with a cryptic error without CDF.
#    File: databricks/ddl.sql

# 2. Create endpoint + index. This is NOT SQL; the exact REST calls are in
#    comments at the bottom of ddl.sql. Roughly:
#    POST /api/2.0/vector-search/endpoints   {"name":"entire-handoff", ...}
#    POST /api/2.0/vector-search/indexes     {"name":"main.default.handoff_idx", ...}

# 3. Point the CLI at it
export DATABRICKS_HOST=https://<workspace>.cloud.databricks.com
export DATABRICKS_TOKEN=<token>
export DATABRICKS_WAREHOUSE_ID=<id>
export DATABRICKS_VOLUME_PATH=/Volumes/main/default/handoff
export DATABRICKS_VECTOR_INDEX=main.default.handoff_idx

# 4. Ingest and query
entire handoff --json | handoff-databricks
entire handoff --ask "how do I mock the checkpoint store"
```

**Two cautions.** A Vector Search endpoint takes ~10 minutes to provision and
**costs money while running**. The table currently holds only the 11 rows from
the sample packet, so retrieval will be thin until real checkpoints accumulate.

Works with no workspace at all:

```bash
DATABRICKS_HOST=file:///tmp/out entire handoff --json | handoff-databricks
```

**MLflow evaluation is not built.** It was in the plan (~20 hand-labelled
`task → expected dead end` pairs, report recall@k). It turns "our retrieval is
relevant" from a claim into a number. Skip it unless there is spare time.

---

## 5. Say only what is true

Built and working: `entire handoff` with five sections, the dead-end miner,
markdown + JSON output, the managed skill, Delta ingest, and the retrieval code
path (tested against a fake server, **never against a live index**).

Not built: **MLflow eval**, the **MCP tool** (`grep entire_handoff
cmd/entire/cli/mcp.go` → 0), and no Vector Search index has ever been created.

Also untrue and worth not repeating — these appeared in an earlier project
write-up and were checked against the code:

- "requires CGO because of tree-sitter" — **there is no tree-sitter in `go.mod`**.
- "uses the Databricks Go SDK" — **`grep databricks go.mod` → 0**. Plain `net/http`.
- Extractors "mutate the Packet" — they return a `Section`; `Build` assembles.

---

## 6. Checkpoint status

**3 real checkpoints exist**, all resolving under `entire checkpoint explain`:
`b35246429`, `20b1d9c59`, `aaf097182`.

Four are needed, so **one more commit on a machine with working hooks closes
it.** The six commits before the PATH fix have no session data behind them and
cannot be reconstructed.

---

## Order I would work in

1. Fix the push permission and get the 5 commits onto the remote. (§0)
2. Set up Linux, verify hooks capture. (§1)
3. Capture the three graph artefacts. (§2) — cheapest points on the board.
4. Write `BUILDATHON.md`. (§3) — required, and the commit makes checkpoint #4.
5. Databricks index, only if a workspace is ready. (§4)
6. MLflow, only if everything else is done. (§4)
