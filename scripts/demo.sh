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
DIM="\033[2m"
RESET="\033[0m"

step() {
    echo -e "\n${BOLD}${CYAN}======================================================================${RESET}"
    echo -e "${BOLD}${CYAN}▶ STEP $1: $2${RESET}"
    echo -e "${BOLD}${CYAN}======================================================================${RESET}\n"
}

pause() {
    echo -e "${DIM}Press Enter to continue...${RESET}"
    read -r
}

# Auto-mode flag if passed non-interactively
AUTO=false
if [[ "${1:-}" == "--auto" || "${1:-}" == "-y" || ! -t 0 ]]; then
    AUTO=true
    pause() { sleep 1.5; }
fi

clear 2>/dev/null || true

echo -e "${BOLD}${MAGENTA}======================================================================${RESET}"
echo -e "${BOLD}${MAGENTA}             🚀 ENTIRE HANDOFF — DEMO SHOWCASE 🚀                     ${RESET}"
echo -e "${BOLD}${MAGENTA}       Checkpoint-Native Developer Experience for AI Pair Coding      ${RESET}"
echo -e "${BOLD}${MAGENTA}======================================================================${RESET}"
echo -e "${DIM}Track 1: Build a Checkpoint-Native Developer Experience${RESET}\n"
echo -e "This demo showcases how ${BOLD}entire handoff${RESET} transforms native git checkpoints"
echo -e "into cited, zero-friction orientation briefings for humans and agents alike.\n"
pause

# Step 1: Checkpoint Native Storage
step "1" "Checkpoint Storage & Status"
echo -e "${YELLOW}Entire CLI is enabled, capturing developer & AI sessions as native Git refs:${RESET}\n"
entire status
echo -e "\n${YELLOW}Native Git Checkpoint References in .git:${RESET}"
git for-each-ref refs/entire/checkpoints/ | head -n 5
echo -e "${DIM}... and more synced directly to origin.${RESET}\n"
pause

# Step 2: Live Handoff Generation
step "2" "Live 'entire handoff' Generation"
echo -e "${YELLOW}Running 'entire handoff' to build a real-time briefing from repository checkpoints:${RESET}\n"
entire handoff --limit 5 || true
echo -e "\n${GREEN}✓ Notice the citations [01M1...] linking every single claim to a verifiable checkpoint!${RESET}\n"
pause

# Step 3: Dead-End Mining & Rich Packet Inspection
step "3" "Dead-End Mining & Sample Handoff Packet"
echo -e "${YELLOW}Entire Handoff extracts failed tool calls and dead ends from transcript.jsonl:${RESET}"
echo -e "${DIM}(No git diff captures what an agent tried and failed to do — Entire Checkpoints do!)${RESET}\n"

if [ -f "cmd/entire/cli/handoff/testdata/sample_packet.json" ]; then
    echo -e "${BOLD}Sample Packet Dead-End Extraction:${RESET}"
    python3 -c "
import json
with open('cmd/entire/cli/handoff/testdata/sample_packet.json') as f:
    p = json.load(f)
for s in p['sections']:
    if s['name'] == 'dead_ends':
        for it in s['items']:
            print(f'  ❌ {it[\"text\"]}')
            print(f'     ↳ Cites: {it[\"cites\"][0][\"checkpoint_id\"]} (line {it[\"cites\"][0].get(\"line\",\"-\")})\n')
"
fi
pause

# Step 4: Machine-Readable JSON for Agent Ingestion
step "4" "Machine-Readable JSON Output (--json)"
echo -e "${YELLOW}Fresh AI agent sessions invoke 'entire handoff --json' to orient before editing files:${RESET}\n"
entire handoff --limit 2 --json | head -n 25 || true
echo -e "\n${DIM}... full structured JSON packet provided for zero-shot agent orientation.${RESET}\n"
pause

# Step 5: Databricks & Delta Lake Pipeline
step "5" "Databricks Delta Lake Telemetry & Ingestion"
echo -e "${YELLOW}The 'handoff-databricks' tool ingests handoff packets into Delta tables:${RESET}\n"
if [ -f "cmd/handoff-databricks/testdata/packet.json" ]; then
    echo -e "${BOLD}Databricks Schema & Ingestion Mapping:${RESET}"
    echo -e " - Table: ${CYAN}entire_handoff_packets${RESET} (UUID, Repo, Checkpoint ID, Generated Time)"
    echo -e " - Table: ${CYAN}entire_handoff_items${RESET} (Section, Content, Citing Checkpoint)"
    echo -e " - DDL:    ${CYAN}databricks/ddl.sql${RESET}"
fi
echo ""
pause

# Step 6: Summary & Highlights
step "6" "Demo Summary & Key Highlights"
echo -e "${BOLD}${GREEN}Why Entire Handoff Wins:${RESET}"
echo -e " 1. ${BOLD}100% Checkpoint-Native${RESET} — Every fact is grounded in git-backed session checkpoints."
echo -e " 2. ${BOLD}Zero Re-Work${RESET} — Mines dead ends so agents don't re-try already-failed approaches."
echo -e " 3. ${BOLD}Verifiable Citations${RESET} — Every bullet links to a checkpoint inspectable via 'entire cp explain'."
echo -e " 4. ${BOLD}Dual-Mode Output${RESET} — Markdown for humans, JSON for agents and automated pipelines."
echo -e " 5. ${BOLD}Enterprise Ready${RESET} — Out-of-the-box ingestion to Databricks Unity Catalog & Delta Lake."
echo -e "\n${BOLD}${MAGENTA}======================================================================${RESET}"
echo -e "${BOLD}${MAGENTA}                   Demo Complete! Ready for Evaluation               ${RESET}"
echo -e "${BOLD}${MAGENTA}======================================================================${RESET}\n"
