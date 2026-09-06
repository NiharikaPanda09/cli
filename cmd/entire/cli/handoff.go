package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/go-git/go-git/v6"
	"github.com/spf13/cobra"

	"github.com/entireio/cli/cmd/entire/cli/checkpoint"
	"github.com/entireio/cli/cmd/entire/cli/gitremote"
	"github.com/entireio/cli/cmd/entire/cli/gitrepo"
	"github.com/entireio/cli/cmd/entire/cli/handoff"
	"github.com/entireio/cli/cmd/entire/cli/paths"
	"github.com/entireio/cli/cmd/entire/cli/strategy"
	"github.com/entireio/cli/cmd/entire/cli/trailers"
)

type handoffFlags struct {
	checkpoint string
	format     string
	limit      int
	noGraph    bool
}

const (
	handoffFormatMarkdown = "md"
	handoffFormatJSON     = "json"
)

func newHandoffCmd() *cobra.Command {
	f := &handoffFlags{format: handoffFormatMarkdown, limit: handoff.DefaultLimit}
	cmd := &cobra.Command{
		Use:   "handoff",
		Short: "Build a cited handoff packet for the next session",
		Long: "Build a handoff packet summarizing recent checkpoints: what the last sessions\n" +
			"were trying to do, what is still open, what was already tried and failed, and\n" +
			"where work stopped.\n\n" +
			"Every line carries the checkpoint it came from, so each claim can be traced\n" +
			"back to the session that produced it.\n\n" +
			"The --json output is a stable, documented shape intended for other tools.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// --json is the conventional spelling across this CLI; --format is
			// kept for symmetry with the other renderers. Either selects JSON.
			if jsonFlag, err := cmd.Flags().GetBool("json"); err == nil && jsonFlag {
				f.format = handoffFormatJSON
			}
			switch f.format {
			case handoffFormatMarkdown, handoffFormatJSON:
			default:
				cmd.SilenceUsage = true
				return fmt.Errorf("unknown format %q: want %q or %q", f.format, handoffFormatMarkdown, handoffFormatJSON)
			}
			return runHandoff(cmd.Context(), cmd.OutOrStdout(), f)
		},
	}
	cmd.Flags().StringVar(&f.checkpoint, "checkpoint", "", "Start from this checkpoint id instead of the newest")
	cmd.Flags().StringVar(&f.format, "format", handoffFormatMarkdown, "Output format: md or json")
	cmd.Flags().IntVar(&f.limit, "limit", handoff.DefaultLimit, "Maximum checkpoints to read")
	cmd.Flags().BoolVar(&f.noGraph, "no-graph", false, "Skip the graph-derived blast-radius section")
	cmd.Flags().Bool("json", false, "Shorthand for --format json")
	return cmd
}

func runHandoff(ctx context.Context, out io.Writer, f *handoffFlags) error {
	repo, err := gitrepo.OpenCurrent(ctx)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}
	defer repo.Close()

	stores, err := checkpoint.Open(ctx, repo, checkpoint.OpenOptions{
		BlobFetcher: FetchBlobsByHash,
		RefFetcher:  FetchCheckpointRef,
		ReadRemotes: strategy.CheckpointReadRemotes(ctx),
	})
	if err != nil {
		return fmt.Errorf("open checkpoint store: %w", err)
	}

	in, err := handoff.Load(ctx, stores.Persistent, handoff.LoadOptions{
		Repo:       handoffRepoName(ctx),
		Head:       handoffHeadCheckpoint(ctx, repo),
		Limit:      f.limit,
		Checkpoint: f.checkpoint,
	})
	if err != nil {
		return fmt.Errorf("read checkpoints: %w", err)
	}

	packet := handoff.Build(ctx, in, f.noGraph)

	if f.format == handoffFormatJSON {
		return handoff.RenderJSON(out, packet)
	}
	return handoff.RenderMarkdown(out, packet)
}

// handoffRepoName identifies the repo for the packet header.
//
// Resolved from the origin remote, falling back to the worktree's base name.
// Deliberately local-only: currentRepoRef would answer more precisely but calls
// the control plane, and a handoff must still build offline.
func handoffRepoName(ctx context.Context) string {
	forge, owner, repo, err := gitremote.ResolveRemoteRepo(ctx, "origin")
	if err == nil && owner != "" && repo != "" {
		if forge != "" {
			return fmt.Sprintf("%s/%s/%s", forge, owner, repo)
		}
		return owner + "/" + repo
	}
	if root, err := paths.WorktreeRoot(ctx); err == nil {
		return "local/" + filepath.Base(root)
	}
	return "local"
}

// handoffHeadCheckpoint returns the checkpoint ID on HEAD's Entire-Checkpoint
// trailer, or "" when HEAD carries none. Absence is normal -- the working tree
// may simply be ahead of the last checkpointed commit -- so it is never an error.
func handoffHeadCheckpoint(ctx context.Context, repo *git.Repository) string {
	head, err := repo.Head()
	if err != nil {
		return ""
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return ""
	}
	if cpID, ok := trailers.ParseCheckpoint(commit.Message); ok {
		return cpID.String()
	}
	return ""
}
