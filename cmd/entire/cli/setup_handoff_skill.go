package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/entireio/cli/cmd/entire/cli/agent"
	"github.com/entireio/cli/cmd/entire/cli/agent/types"
	"github.com/entireio/cli/cmd/entire/cli/paths"
)

const entireManagedHandoffSkillMarker = "ENTIRE-MANAGED HANDOFF SKILL v1"

func setupOptionalHandoffSkill(ctx context.Context, w io.Writer, ag agent.Agent, opts EnableOptions) error {
	if !opts.HandoffSkill {
		return nil
	}
	result, err := scaffoldHandoffSkill(ctx, ag)
	if err != nil {
		return fmt.Errorf("failed to scaffold %s handoff skill: %w", ag.Name(), err)
	}
	reportHandoffSkillScaffold(w, ag, result)
	return nil
}

func reportHandoffSkillScaffold(w io.Writer, ag agent.Agent, result managedScaffoldResult) {
	switch result.Status {
	case managedScaffoldCreated:
		fmt.Fprintf(w, "  ✓ Installed %s handoff skill\n", ag.Type())
		fmt.Fprintf(w, "    %s\n", result.RelPath)
	case managedScaffoldUpdated:
		fmt.Fprintf(w, "  ✓ Updated %s handoff skill\n", ag.Type())
		fmt.Fprintf(w, "    %s\n", result.RelPath)
	case managedScaffoldSkippedConflict:
		fmt.Fprintf(w, "  Skipped %s handoff skill (unmanaged file exists)\n", ag.Type())
		fmt.Fprintf(w, "    %s\n", result.RelPath)
	case managedScaffoldUnsupported:
		fmt.Fprintf(w, "  Handoff skill is not supported for %s\n", ag.Type())
	case managedScaffoldUnchanged:
		fmt.Fprintf(w, "  Handoff skill already installed for %s\n", ag.Type())
		fmt.Fprintf(w, "    %s\n", result.RelPath)
	}
}

func setupOptionalHandoffSkillForNames(ctx context.Context, w io.Writer, names []string, opts EnableOptions) error {
	return setupOptionalSkillForNames(ctx, w, names, opts.HandoffSkill, setupOptionalHandoffSkill, opts)
}

func scaffoldHandoffSkill(ctx context.Context, ag agent.Agent) (managedScaffoldResult, error) {
	relPath, content, ok := handoffSkillTemplate(ag.Name())
	if !ok {
		return managedScaffoldResult{Status: managedScaffoldUnsupported}, nil
	}

	// Anchored on the worktree root, never the current directory: relPath names
	// a file under an agent's own directory, and writing that beside the process
	// instead of in the repository is the mistake, not the fallback.
	repoRoot, err := paths.WorktreeRoot(ctx)
	if err != nil {
		return managedScaffoldResult{}, fmt.Errorf("resolve worktree root: %w", err)
	}
	root, err := openScaffoldRoot(repoRoot)
	if err != nil {
		return managedScaffoldResult{}, err
	}
	return writeManagedScaffold(root, relPath, content, isManagedHandoffSkill)
}

func isManagedHandoffSkill(data []byte) bool {
	return bytes.Contains(data, []byte(entireManagedHandoffSkillMarker))
}

// handoffSkillTemplate returns the skill file for an agent, or ok=false when
// the agent has no skills directory we manage.
func handoffSkillTemplate(agentName types.AgentName) (string, []byte, bool) {
	var root string
	switch agentName {
	case agent.AgentNameClaudeCode:
		root = ".claude"
	case agent.AgentNameCodex:
		root = ".agents"
	case agent.AgentNameGemini:
		root = ".gemini"
	default:
		// Deliberately narrow: the packet is only as good as the transcripts
		// behind it, and those are reliably shaped for Claude Code today.
		return "", nil, false
	}
	relPath := filepath.Join(root, "skills", "entire-handoff", "SKILL.md")
	return relPath, []byte(handoffSkillBody), true
}

// handoffSkillBody is deliberately one instruction.
//
// The claim this product makes is "a fresh agent orients itself in a single
// call". A skill that offers five options does not test that claim; a skill
// that says "run this first, before you read anything" does.
const handoffSkillBody = `---
name: entire-handoff
description: >-
  Load context from previous coding sessions in this repo. Use at the START of a
  session, before reading files, whenever you are picking up work someone (or
  some earlier session) already began.
---

<!-- ` + entireManagedHandoffSkillMarker + ` -->

# Resuming work in this repo

**Run this first, before reading any file:**

` + "```bash" + `
entire handoff --json
` + "```" + `

It returns a packet assembled from this repo's checkpoints, with five sections:

| Section | What it tells you |
| --- | --- |
| ` + "`intent`" + ` | What the previous sessions were trying to do |
| ` + "`open_items`" + ` | Promises made and not yet kept. Items marked ` + "`(likely closed)`" + ` were probably resolved by later work — verify before assuming either way |
| ` + "`dead_ends`" + ` | Approaches already tried that FAILED. **Do not repeat these.** |
| ` + "`surface`" + ` | Blast radius of what was touched |
| ` + "`stopped`" + ` | Where the last session left off |

Every item carries a ` + "`cites`" + ` array of checkpoint IDs. To read the full session
behind any claim:

` + "```bash" + `
entire checkpoint explain <checkpoint_id>
` + "```" + `

## How to use it

1. Read ` + "`dead_ends`" + ` before proposing an approach. It is the section that saves
   the most time — it is a record of what has already been ruled out here.
2. Treat ` + "`open_items`" + ` as the backlog you inherited.
3. Cite the checkpoint ID when you tell the user why you are doing something.
   The point of the packet is that every claim is traceable; keep it that way.

A section may be empty with a ` + "`note`" + ` explaining why (for example, no summaries
were generated in range). That is normal — use the sections you did get.
`
