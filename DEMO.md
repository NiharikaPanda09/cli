# Entire Handoff — Demo Guide & Speaker Notes

> **Track 1: Build a Checkpoint-Native Developer Experience**  
> *Turn Entire Checkpoints into cited, high-trust handoff briefings so developers and AI agents resume work oriented instead of blind.*

---

## 🎬 How to Run the Demo

### Interactive Mode (Recommended for live presentations)
```bash
./scripts/demo.sh
```
*Pauses after each step with clear explanations. Press `Enter` to advance.*

### Automated Mode (Fast / Non-interactive)
```bash
./scripts/demo.sh --auto
```

---

## 🔍 Step-by-Step Walkthrough & Explanations

### Step 1: Checkpoint Storage & Status
- **What it does:** Displays `entire status` and lists references under `.git/refs/entire/checkpoints/`.
- **Explanation:** Entire stores all agent session transcripts, tool calls, and diffs as native Git objects directly inside your repository. No proprietary SaaS silo or external database is required for core functionality. Checkpoints travel alongside your code during `git push` and `git pull`.

---

### Step 2: Live `entire handoff` Generation
- **What it does:** Runs `entire handoff --limit 5` on the live repository.
- **Explanation:** Synthesizes an orientation briefing in seconds:
  1. **What we were trying to do (Intent)** — Session goals extracted from summaries.
  2. **Still Open** — Unresolved tasks and promises.
  3. **Already Tried (Dead Ends)** — Approaches that failed.
  4. **Blast Radius** — Entity impact analysis from `entire graph diff`.
  5. **Where it Stopped** — Current HEAD checkpoint, attribution, token counts.
  6. **Citations (`[01M1...]`)** — Every single line is linked to a verifiable checkpoint ID.

---

### Step 3: Dead-End Mining (The Core Innovation)
- **What it does:** Inspects `transcript.jsonl` error blocks from sample packets.
- **Explanation:** When an agent attempts an approach and fails (e.g., incorrect flag, missing dependency, failing test), it pivots. Standard git diffs discard this history completely. Entire Checkpoints preserve `result.status == "error"` along with the agent's pivot reason. Surfacing this prevents incoming developers and agents from wasting time and tokens repeating the exact same failed attempts.

---

### Step 4: Machine-Readable JSON for AI Agents
- **What it does:** Runs `entire handoff --json`.
- **Explanation:** Fresh agent sessions bootstrap themselves using `entire enable --handoff-skill`. Before reading files or guessing architecture, the agent consumes the structured JSON packet to gain immediate context about recent changes, open blockers, and failed experiments.

---

### Step 5: Databricks Delta Lake Telemetry
- **What it does:** Demonstrates `handoff-databricks` ingestion and schema mapping from [`databricks/ddl.sql`](databricks/ddl.sql).
- **Explanation:** Transforms handoff packets into Delta Lake tables (`entire_handoff_packets`, `entire_handoff_items`) in Databricks Unity Catalog. Engineering teams can query recurring blockers across hundreds of repositories and track AI development velocity.

---

### Step 6: Summary & Evaluation Highlights
- **Why Entire Handoff Wins Track 1:**
  1. **100% Checkpoint-Native:** Grounded in Git-backed Entire session checkpoints.
  2. **Zero Re-Work:** Eliminates repetitive debugging loops.
  3. **Verifiable Provenance:** Every claim is backed by an immutable checkpoint ID.
  4. **Dual-Mode Output:** Human-readable Markdown and machine-parsable JSON.
  5. **Fast & Offline:** Runs in milliseconds with zero required external API keys.
