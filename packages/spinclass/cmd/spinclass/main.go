package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/amarbel-llc/spinclass/internal/clean"
	"github.com/amarbel-llc/spinclass/internal/completions"
	"github.com/amarbel-llc/spinclass/internal/executor"
	"github.com/amarbel-llc/spinclass/internal/merge"
	"github.com/amarbel-llc/spinclass/internal/perms"
	"github.com/amarbel-llc/spinclass/internal/pull"
	"github.com/amarbel-llc/spinclass/internal/shop"
	"github.com/amarbel-llc/spinclass/internal/status"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)

var outputFormat string
var createVerbose bool

var rootCmd = &cobra.Command{
	Use:   "spinclass",
	Short: "Shell-agnostic git worktree session manager",
	Long:  `spinclass manages git worktree lifecycles: creating worktrees + sessions, and offering close workflows (rebase, merge, cleanup, push).`,
}

var createCmd = &cobra.Command{
	Use:   "create <target>",
	Short: "Create a worktree without attaching",
	Long:  `Create a new worktree and apply sweatfile settings. Does not start a session. Target is a branch name or path, resolved relative to the current git repository.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoPath, err := worktree.DetectRepo(cwd)
		if err != nil {
			return err
		}

		rp, err := worktree.ResolvePath(repoPath, args[0])
		if err != nil {
			return err
		}

		return shop.Create(rp, createVerbose)
	},
}

var attachCmd = &cobra.Command{
	Use:     "attach <target> [claude args...]",
	Aliases: []string{"open"},
	Short:   "Create (if needed) and attach to a worktree session",
	Long:    `Create a worktree if it doesn't exist, then attach to a session. Target is a branch name or path, resolved relative to the current git repository. If additional arguments are provided, claude is launched with those arguments instead of a shell.`,
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format := outputFormat
		if format == "" {
			format = "tap"
		}

		exec := executor.ShellExecutor{}

		var claudeArgs []string
		if len(args) >= 2 {
			claudeArgs = args[1:]
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoPath, err := worktree.DetectRepo(cwd)
		if err != nil {
			return err
		}

		rp, err := worktree.ResolvePath(repoPath, args[0])
		if err != nil {
			return err
		}

		return shop.Attach(exec, rp, format, claudeArgs)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all repos and worktrees",
	Long:  `Scan the current directory (or repo) for worktrees and display a styled table showing branch status, dirty state, remote tracking, and modification dates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		format := outputFormat
		if format == "" {
			format = "table"
		}

		rows := status.CollectStatus(cwd)
		if len(rows) == 0 {
			log.Info("no repos found")
			return nil
		}

		if format == "tap" {
			status.RenderTap(rows, os.Stdout)
		} else {
			fmt.Println(status.Render(rows))
		}
		return nil
	},
}

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge current worktree into main",
	Long:  `Run from inside a worktree. Merges the worktree branch into the main repo with --ff-only and removes the worktree.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return merge.Run(executor.ShellExecutor{})
	},
}

var cleanInteractive bool

var pullDirty bool

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull repos and rebase worktrees",
	Long:  `Pull all clean repos, then rebase all clean worktrees onto their repo's default branch. Use -d to include dirty repos and worktrees.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		return pull.Run(cwd, pullDirty)
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove merged worktrees",
	Long:  `Scan all worktrees, identify those whose branches are fully merged into the main branch, and remove them. Use -i to interactively handle dirty worktrees.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		format := outputFormat
		if format == "" {
			format = "tap"
		}

		return clean.Run(cwd, cleanInteractive, format)
	},
}

var completionsCmd = &cobra.Command{
	Use:    "completions",
	Short:  "Generate tab-separated completions",
	Long:   `Output tab-separated completion entries for shell integration. Scans local worktrees.`,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		completions.Local(cwd, os.Stdout)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&outputFormat, "format", "", "output format: tap or table")
	createCmd.Flags().BoolVarP(&createVerbose, "verbose", "v", false, "print sweatfile loading details")
	cleanCmd.Flags().BoolVarP(&cleanInteractive, "interactive", "i", false, "interactively discard changes in dirty merged worktrees")
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(mergeCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(completionsCmd)
	pullCmd.Flags().BoolVarP(&pullDirty, "dirty", "d", false, "include dirty repos and worktrees")
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(perms.NewPermsCmd())
}

func main() {
	rootCmd.Use = filepath.Base(os.Args[0])
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
