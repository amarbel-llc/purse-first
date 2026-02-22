package perms

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/amarbel-llc/spinclass/internal/worktree"
)

func NewPermsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "perms",
		Short: "Manage Claude Code permission tiers",
	}

	cmd.AddCommand(newCheckCmd())
	cmd.AddCommand(newReviewCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newEditCmd())

	return cmd
}

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "check",
		Short:  "Handle a PermissionRequest hook",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunCheck(os.Stdin, os.Stdout, TiersDir())
		},
	}
}

func newReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review [worktree-path]",
		Short: "Interactively review new permissions from a session",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var worktreePath string
			if len(args) > 0 {
				worktreePath = args[0]
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				worktreePath = cwd
			}

			if !filepath.IsAbs(worktreePath) {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				worktreePath = filepath.Join(cwd, worktreePath)
			}

			repoPath, err := worktree.DetectRepo(worktreePath)
			if err != nil {
				return fmt.Errorf("could not detect repo: %w", err)
			}
			repoName := filepath.Base(repoPath)

			return RunReviewInteractive(worktreePath, repoName)
		},
	}
}

func newListCmd() *cobra.Command {
	var repo string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List permission tier rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			tiersDir := TiersDir()

			globalPath := filepath.Join(tiersDir, "global.json")
			globalTier, err := LoadTierFile(globalPath)
			if err != nil {
				return fmt.Errorf("loading global tier: %w", err)
			}

			fmt.Println("Global tier:")
			if len(globalTier.Allow) == 0 {
				fmt.Println("  (empty)")
			} else {
				for _, rule := range globalTier.Allow {
					fmt.Printf("  %s\n", rule)
				}
			}

			if repo != "" {
				repoPath := filepath.Join(tiersDir, "repos", repo+".json")
				repoTier, err := LoadTierFile(repoPath)
				if err != nil {
					return fmt.Errorf("loading repo tier %s: %w", repo, err)
				}

				fmt.Printf("\nRepo tier (%s):\n", repo)
				if len(repoTier.Allow) == 0 {
					fmt.Println("  (empty)")
				} else {
					for _, rule := range repoTier.Allow {
						fmt.Printf("  %s\n", rule)
					}
				}

				return nil
			}

			reposDir := filepath.Join(tiersDir, "repos")
			entries, err := os.ReadDir(reposDir)
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}

			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
					continue
				}

				repoName := strings.TrimSuffix(entry.Name(), ".json")
				repoTier, err := LoadTierFile(filepath.Join(reposDir, entry.Name()))
				if err != nil {
					continue
				}

				if len(repoTier.Allow) == 0 {
					continue
				}

				fmt.Printf("\nRepo tier (%s):\n", repoName)
				for _, rule := range repoTier.Allow {
					fmt.Printf("  %s\n", rule)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "show rules for a specific repo only")

	return cmd
}

func newEditCmd() *cobra.Command {
	var global bool
	var repo string

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit a permission tier file in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			tiersDir := TiersDir()

			var tierPath string
			if global {
				tierPath = filepath.Join(tiersDir, "global.json")
			} else if repo != "" {
				tierPath = filepath.Join(tiersDir, "repos", repo+".json")
			} else {
				tierPath = filepath.Join(tiersDir, "global.json")
			}

			if _, err := os.Stat(tierPath); os.IsNotExist(err) {
				if err := SaveTierFile(tierPath, Tier{Allow: []string{}}); err != nil {
					return fmt.Errorf("creating tier file: %w", err)
				}
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			editorCmd := exec.Command(editor, tierPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			return editorCmd.Run()
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "edit the global tier file")
	cmd.Flags().StringVar(&repo, "repo", "", "edit a repo-specific tier file")

	return cmd
}

func RunReviewInteractive(worktreePath, repoName string) error {
	settingsPath := filepath.Join(worktreePath, ".claude", "settings.local.json")
	snapshotPath := filepath.Join(worktreePath, ".claude", ".settings-snapshot.json")

	snapshot, err := LoadClaudeSettings(snapshotPath)
	if err != nil {
		return err
	}

	current, err := LoadClaudeSettings(settingsPath)
	if err != nil {
		return err
	}

	newRules := DiffRules(snapshot, current)
	if len(newRules) == 0 {
		return nil
	}

	tiersDir := TiersDir()
	var decisions []ReviewDecision

	for _, rule := range newRules {
		var action string

		selectPrompt := huh.NewSelect[string]().
			Title(fmt.Sprintf("New permission: %s", rule)).
			Options(
				huh.NewOption("Promote to global (all repos)", ReviewPromoteGlobal),
				huh.NewOption(fmt.Sprintf("Promote to %s (this repo)", repoName), ReviewPromoteRepo),
				huh.NewOption("Keep for this worktree only", ReviewKeep),
				huh.NewOption("Discard", ReviewDiscard),
			).
			Value(&action)

		if err := selectPrompt.Run(); err != nil {
			return err
		}

		decisions = append(decisions, ReviewDecision{
			Rule:   rule,
			Action: action,
		})
	}

	return RouteDecisions(tiersDir, repoName, settingsPath, decisions)
}
