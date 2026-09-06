#!/usr/bin/env bash
# Entire Handoff - End-to-End Demo Showcase
# Demonstrates Track 1 (Checkpoint-Native Developer Experience)

set -euo pipefail

# ANSI color codes
BOLD="\033[1m"
CYAN="\033[36m"
GREEN="\033[32m"
YELLOW="\033[33m"
BLUE="\033[34m"
MAGENTA="\033[35m"
RED="\033[31m"
DIM="\033[2m"
RESET="\033[0m"

step_header() {
    local num="$1"
    local title="$2"
    local summary="$3"
    echo -e "\n${BOLD}${CYAN}╔══════════════════════════════════════════════════════════════════════╗${RESET}"
    echo -e "${BOLD}${CYAN}║  STEP ${num}: ${title}${RESET}"
    echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════════════════════════════════╝${RESET}"
    echo -e "${DIM}💡 EXPLANATION:${RESET} ${summary}\n"
}

explain_box() {
    local why="$1"
    echo -e "\n${BLUE}┌────────────────────────── 🔍 WHY THIS MATTERS ──────────────────────────┐${RESET}"
    echo -e "${BLUE}│${RESET} ${why}"
    echo -e "${BLUE}└─────────────────────────────────────────────────────────────────────────┘${RESET}\n"
}

pause() {
    echo -e "${DIM}Press [Enter] to proceed to next step...${RESET}"
    read -r
}

# Auto-mode flag if passed non-interactively
AUTO=false
if [[ "${1:-}" == "--auto" || "${1:-}" == "-y" || ! -t 0 ]]; then
    AUTO=true
    pause() { sleep 2; }
fi

clear 2>/dev/null || true

echo -e "${BOLD}${MAGENTA}════════════════════════════════════════════════════════════════════════${RESET}"
echo -e "${BOLD}${MAGENTA}             🚀 ENTIRE HANDOFF — DEMO SHOWCASE 🚀                       ${RESET}"
echo -e "${BOLD}${MAGENTA}       Checkpoint-Native Developer Experience for AI Pair Coding        ${RESET}"
echo -e "${BOLD}${MAGENTA}════════════════════════════════════════════════════════════════════════${RESET}"
echo -e "${DIM}Track 1: Build a Checkpoint-Native Developer Experience${RESET}"
echo -e "\n${BOLD}The Problem:${RESET} Every new AI coding session starts with zero memory."
echo -e "It re-tries approaches that already failed and drops promises made in previous turns."
echo -e "Git diffs only tell you what *changed* — never what was *attempted and abandoned*."
echo -e "\n${BOLD}The Solution:${RESET} ${GREEN}entire handoff${RESET} mines Git-backed Entire Checkpoints to"
echo -e "generate cited, high-trust orientation briefings for humans and agents alike.\n"
pause

# ==============================================================================
# Step 1: Checkpoint Native Storage
# ==============================================================================
step_header "1" "Checkpoint Storage & Status" \
    "Entire stores developer & agent sessions as first-class Git references inside .git."

echo -e "${YELLOW}Running 'entire status' to inspect enabled agents and sync target:${RESET}"
entire status

echo -e "\n${YELLOW}Inspecting native Git references in 'refs/entire/checkpoints/':${RESET}"
git for-each-ref refs/entire/checkpoints/ | head -n 5
echo -e "${DIM}... (Checkpoints live right alongside Git commits, preventing SaaS lock-in).${RESET}"

explain_box "Because checkpoints are stored natively in Git, they travel with 'git push' and\n 'git clone'. No external cloud database is required for core functionality."
pause

# ==============================================================================
# Step 2: Live Handoff Generation
# ==============================================================================
step_header "2" "Live 'entire handoff' Generation" \
    "Synthesizing an instant briefing from recent checkpoints with verifiable citations."

echo -e "${YELLOW}Executing 'entire handoff' on the current repository:${RESET}\n"
entire handoff --limit 5 || true

explain_box "Notice the citation tags (e.g. [01M1...]). Every bullet links directly to a real\n checkpoint that can be inspected with 'entire checkpoint explain <id>'."
pause

# ==============================================================================
# Step 3: Dead-End Mining
# ==============================================================================
step_header "3" "Dead-End Mining (The Killer Feature)" \
    "Mines 'result.status == error' from transcript.jsonl to surface failed approaches."

echo -e "${YELLOW}Demonstrating dead-end extraction from session transcripts:${RESET}"
echo -e "${DIM}When an agent runs a command that fails, it pivots. Git diffs throw that away.${RESET}"
echo -e "${DIM}Entire Handoff extracts the failure and the agent's pivot reason:${RESET}\n"

if [ -f "cmd/entire/cli/handoff/testdata/sample_packet.json" ]; then
    python3 -c "
import json
with open('cmd/entire/cli/handoff/testdata/sample_packet.json') as f:
    p = json.load(f)
for s in p['sections']:
    if s['name'] == 'dead_ends':
        for it in s['items']:
            print(f'  ❌ {it[\"text\"]}')
            print(f'     ↳ Provenance: Checkpoint {it[\"cites\"][0][\"checkpoint_id\"]} (line {it[\"cites\"][0].get(\"line\",\"-\")})\n')
"
fi

explain_box "Dead ends save massive time and token costs. A fresh agent immediately knows\n what NOT to do instead of repeating the same mistakes."
pause

# ==============================================================================
# Step 4: Machine-Readable JSON for AI Agents
# ==============================================================================
step_header "4" "Machine-Readable JSON Output (--json)" \
    "Enables AI agents to bootstrap their context window zero-shot before touching files."

echo -e "${YELLOW}Executing 'entire handoff --json' for agent consumption:${RESET}\n"
entire handoff --limit 2 --json | head -n 30 || true
echo -e "\n${DIM}... (Structured schema documented in DOCUMENTATION.md).${RESET}"

echo -e "\n${YELLOW}How Agents Auto-Orient:${RESET}"
echo -e "Running ${CYAN}entire enable --handoff-skill${RESET} installs a pre-session agent skill."
echo -e "Before touching any code files, the agent runs 'entire handoff --json' to orient."

explain_box "Dual-mode rendering: Markdown is optimized for human developers reading a terminal,\n while JSON provides a clean, typed AST for automated agents and IDE plugins."
pause

# ==============================================================================
# Step 5: Databricks & Delta Lake Ingestion
# ==============================================================================
step_header "5" "Databricks Delta Lake Telemetry & Ingestion" \
    "Streaming handoff packets into Unity Catalog & Delta tables for organizational intelligence."

echo -e "${YELLOW}Databricks Ingestion Tool ('handoff-databricks'):${RESET}\n"
if [ -f "cmd/handoff-databricks/testdata/packet.json" ]; then
    echo -e " - ${BOLD}Source Packet:${RESET} cmd/handoff-databricks/testdata/packet.json"
    echo -e " - ${BOLD}Target Tables:${RESET} entire_handoff_packets, entire_handoff_items"
    echo -e " - ${BOLD}SQL DDL:${RESET}       databricks/ddl.sql"
fi

echo -e "\n${DIM}Simulating packet row transformation:${RESET}"
python3 -c "
import json
with open('cmd/handoff-databricks/testdata/packet.json') as f:
    p = json.load(f)
print(f'  • Packet UUID: {p.get(\"head_checkpoint_id\")} -> Table: entire_handoff_packets')
for s in p.get('sections', []):
    print(f'  • Section \"{s[\"name\"]}\" ({len(s.get(\"items\",[]))} items) -> Table: entire_handoff_items')
"

explain_box "By loading handoff packets into Delta Lake, engineering leaders gain cross-repo\n visibility into AI agent productivity, recurring dead ends, and open blockers."
pause

# ==============================================================================
# Step 6: Summary & Evaluation Criteria
# ==============================================================================
step_header "6" "Summary & Evaluation Highlights" \
    "Why Entire Handoff fulfills Track 1 (Checkpoint-Native Developer Experience)."

echo -e "${BOLD}${GREEN}Core Achievements:${RESET}"
echo -e " 1. ${BOLD}100% Checkpoint-Native:${RESET} Grounded in Git-backed Entire session checkpoints."
echo -e " 2. ${BOLD}Dead-End Mining:${RESET} Prevents costly AI loops by surfacing failed approaches."
echo -e " 3. ${BOLD}Verifiable Provenance:${RESET} Every bullet has an immutable checkpoint citation."
echo -e " 4. ${BOLD}Dual-Mode Output:${RESET} Human Markdown + Agent JSON + Databricks Delta ingestion."
echo -e " 5. ${BOLD}Zero External Latency:${RESET} Fast, deterministic, and offline-capable."

echo -e "\n${BOLD}${MAGENTA}════════════════════════════════════════════════════════════════════════${RESET}"
echo -e "${BOLD}${MAGENTA}                   Demo Complete! Ready for Evaluation                 ${RESET}"
echo -e "${BOLD}${MAGENTA}════════════════════════════════════════════════════════════════════════${RESET}\n"
