# Entire Handoff — Demo Guide

> **Track 1: Build a Checkpoint-Native Developer Experience**  
> *Turn Entire Checkpoints into cited, high-trust handoff briefings so developers and agents resume work oriented instead of blind.*

---

## 🎬 Quick-Start Demo (1 Command)

To run the interactive terminal demo:

```bash
./scripts/demo.sh
```

Or non-interactive automated mode:

```bash
./scripts/demo.sh --auto
```

---

## 🔍 Step-by-Step Demo Walkthrough

### 1. Show Checkpoint-Native Storage
Checkpoints live directly inside Git, not in a centralized SaaS silo:
```bash
# Check Entire status and enabled hooks
entire status

# Inspect native Git checkpoint references
git for-each-ref refs/entire/checkpoints/
```

### 2. Run `entire handoff` (Human-Readable Markdown)
Build an instant briefing from recent session checkpoints:
```bash
entire handoff
```
**Key Highlights to point out:**
- **Intent**: What previous sessions were trying to achieve.
- **Still Open**: Tasks, tests, or questions left unresolved.
- **Already Tried (Dead Ends)**: Approaches attempted by AI agents that failed, with tool error logs and pivot rationale.
- **Blast Radius**: Impact surface analysis from `entire graph diff`.
- **Where it Stopped**: Last touched files, agent attribution ratio, token usage.
- **Verifiable Citations**: Every claim has a `[01M1...]` tag linking directly to a real checkpoint.

### 3. Verify Any Citation
Drill into any cited session claim using Entire CLI:
```bash
entire checkpoint explain <checkpoint-id>
```

### 4. Agent Ingestion (`entire handoff --json`)
Show how fresh AI coding sessions bootstrap their context window:
```bash
entire handoff --json
```
- Can be installed as a pre-session skill:
  ```bash
  entire enable --handoff-skill
  ```

### 5. Databricks & Delta Lake Ingestion (`handoff-databricks`)
Ingest handoff packets into Unity Catalog & Delta tables for organizational analytics:
```bash
# Ingest handoff packet to Databricks Delta Lake
./handoff-databricks --packet cmd/handoff-databricks/testdata/packet.json
```
- SQL Schema DDL: [`databricks/ddl.sql`](databricks/ddl.sql)

---

## 💡 Why This Wins Track 1

| Feature | Git Diffs Alone | Entire Handoff |
| :--- | :--- | :--- |
| **Dead-End Detection** | ❌ None (failed tool calls discarded) | ✅ Mined from checkpoint transcripts |
| **Session Intent** | ⚠️ Only commit messages (often missing) | ✅ Synthesized from agent sessions |
| **Verifiability** | ❌ No session provenance | ✅ Every item cited by checkpoint ID |
| **Blast Radius** | ⚠️ Raw line diffs | ✅ Graph semantic change impact |
| **Agent Bootstrap** | ❌ Re-reads whole repo | ✅ Zero-shot orientation packet |
