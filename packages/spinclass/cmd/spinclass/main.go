package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/amarbel-llc/spinclass/internal/clean"
	"github.com/amarbel-llc/spinclass/internal/completions"
	"github.com/amarbel-llc/spinclass/internal/executor"
	"github.com/amarbel-llc/spinclass/internal/hooks"
	"github.com/amarbel-llc/spinclass/internal/merge"
	"github.com/amarbel-llc/spinclass/internal/perms"
	"github.com/amarbel-llc/spinclass/internal/pull"
	"github.com/amarbel-llc/spinclass/internal/shop"
	"github.com/amarbel-llc/spinclass/internal/status"
	"github.com/amarbel-llc/spinclass/internal/sweatfile"
	"github.com/amarbel-llc/spinclass/internal/validate"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)

var (
	outputFormat    string
	verbose         bool
	newMergeOnClose bool
	newNoAttach     bool
	mergeGitSync    bool
)

var rootCmd = &cobra.Command{
	Use:   "spinclass",
	Short: "Shell-agnostic git worktree session manager",
	Long:  `spinclass manages git worktree lifecycles: creating worktrees + sessions, and offering close workflows (rebase, merge, cleanup, push).`,
}

var newCmd = &cobra.Command{
	Use:   "new [name parts...]",
	Short: "Create (if needed) and attach to a worktree session",
	Long:  `Create a worktree if it doesn't exist, then attach to a session. Name parts are joined into a sanitized branch name (snob-case). If an existing branch matches, it is checked out. If no name is provided, a random name is generated.`,
	Args:  cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		format := outputFormat
		if format == "" {
			format = "tap"
		}

		exec := executor.ZmxExecutor{}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoPath, err := worktree.DetectRepo(cwd)
		if err != nil {
			return err
		}

		hierarchy, err := sweatfile.LoadDefaultHierarchy()
		if err != nil {
			return err
		}

		resolvedPath, err := worktree.ResolvePath(hierarchy.Merged, repoPath, args)
		if err != nil {
			return err
		}

		return shop.New(
			os.Stdout,
			exec,
			resolvedPath,
			format,
			newMergeOnClose,
			newNoAttach,
			verbose,
		)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all repos and worktrees",
	Long:  `Scan the current directory (or repo) for worktrees and display a tree showing branch status, dirty state, remote tracking, modification dates, and active zmx sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		format := outputFormat
		if format == "" {
			format = "table"
		}

		repos := status.CollectStatus(cwd)
		if len(repos) == 0 {
			log.Info("no repos found")
			return nil
		}

		if format == "tap" {
			status.RenderTap(repos, os.Stdout)
		} else {
			fmt.Println(status.Render(repos))
		}
		return nil
	},
}

var mergeCmd = &cobra.Command{
	Use:   "merge [target]",
	Short: "Merge a worktree into main",
	Long:  `Merge a worktree branch into the main repo with --ff-only and remove the worktree. When run from inside a worktree, merges that worktree. When run from the main repo, specify a target or choose interactively.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format := outputFormat
		if format == "" {
			format = "tap"
		}

		var target string
		if len(args) == 1 {
			target = args[0]
		}

		return merge.Run(executor.ShellExecutor{}, format, target, mergeGitSync, verbose)
	},
}

var cleanInteractive bool

var pullDirty bool

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull repos and rebase worktrees",
	Long:  `Pull all clean repos, then rebase all clean worktrees onto their repo's default branch. Use -d to include dirty repos and worktrees.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format := outputFormat
		if format == "" {
			format = "tap"
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		return pull.Run(cwd, pullDirty, format)
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

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List active zmx sessions",
	Long:  `List all active zmx sessions in the sc group.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return executor.ZmxExecutor{}.List()
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

var forkCmd = &cobra.Command{
	Use:   "fork [<new-branch>]",
	Short: "Fork current worktree into a new branch",
	Long:  `Create a new worktree branched from the current worktree's HEAD. If new-branch is omitted, a name is auto-generated as <current-branch>-N. Must be run from inside a spinclass session (SPINCLASS_SESSION must be set). Does not attach to the new session.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		session := os.Getenv("SPINCLASS_SESSION")
		if session == "" {
			return fmt.Errorf("SPINCLASS_SESSION is not set: are you inside a spinclass session?")
		}

		// session is "<repo-dirname>/<branch>"; extract branch as everything after first "/"
		slashIdx := strings.Index(session, "/")
		if slashIdx < 0 {
			return fmt.Errorf("invalid SPINCLASS_SESSION format: %q (expected <repo>/<branch>)", session)
		}
		currentBranch := session[slashIdx+1:]

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoPath, err := worktree.DetectRepo(cwd)
		if err != nil {
			return err
		}

		currentPath := filepath.Join(repoPath, worktree.WorktreesDir, currentBranch)

		if _, err := os.Stat(currentPath); os.IsNotExist(err) {
			return fmt.Errorf("current worktree path %s does not exist; fork requires a standard .worktrees layout", currentPath)
		}

		rp := worktree.ResolvedPath{
			AbsPath:    currentPath,
			RepoPath:   repoPath,
			Branch:     currentBranch,
			SessionKey: session,
		}

		var newBranch string
		if len(args) == 1 {
			newBranch = args[0]
		}

		format := outputFormat
		if format == "" {
			format = "tap"
		}

		return shop.Fork(os.Stdout, rp, newBranch, format)
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the sweatfile hierarchy",
	Long:  `Walk the sweatfile hierarchy from PWD and validate each file for structural and semantic correctness. Outputs TAP-14 with subtests.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		exitCode := validate.Run(os.Stdout, home, cwd)
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return nil
	},
}

var cmdExecClaude = &cobra.Command{
	Use:                "exec-claude [claude args...]",
	Short:              "Executes claude after applying sweatfile settings",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		hierarchy, err := sweatfile.LoadDefaultHierarchy()
		if err != nil {
			return err
		}

		return hierarchy.Merged.ExecClaude(args...)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&outputFormat, "format", "", "output format: tap or table")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "show detailed output (YAML diagnostics on passing test points)")
	newCmd.Flags().BoolVar(&newMergeOnClose, "merge-on-close", false, "auto-merge worktree into default branch on session close")
	newCmd.Flags().BoolVar(&newNoAttach, "no-attach", false, "create worktree but skip attaching (show command that would run)")
	mergeCmd.Flags().BoolVar(&mergeGitSync, "git-sync", false, "pull and push after merge")
	cleanCmd.Flags().BoolVarP(&cleanInteractive, "interactive", "i", false, "interactively discard changes in dirty merged worktrees")
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(mergeCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(completionsCmd)
	pullCmd.Flags().BoolVarP(&pullDirty, "dirty", "d", false, "include dirty repos and worktrees")
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(perms.NewPermsCmd())
	rootCmd.AddCommand(hooks.NewHooksCmd())
	rootCmd.AddCommand(forkCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(cmdExecClaude)
}

func main() {
	rootCmd.Use = filepath.Base(os.Args[0])
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
