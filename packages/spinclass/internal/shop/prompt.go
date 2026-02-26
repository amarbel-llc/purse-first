package shop

import (
	"github.com/charmbracelet/huh"
)

type dirtyAction string

const (
	actionDiscard  dirtyAction = "Discard changes and merge"
	actionReattach dirtyAction = "Reattach to session"
	actionExit     dirtyAction = "Exit without integrating"
)

func promptDirtyAction(branch string) (dirtyAction, error) {
	var action dirtyAction
	err := huh.NewSelect[dirtyAction]().
		Title("Worktree " + branch + " has uncommitted changes").
		Options(
			huh.NewOption(string(actionDiscard), actionDiscard),
			huh.NewOption(string(actionReattach), actionReattach),
			huh.NewOption(string(actionExit), actionExit),
		).
		Value(&action).
		Run()
	return action, err
}

func promptDefaultBranch() (string, error) {
	var selected string
	err := huh.NewSelect[string]().
		Title("Both main and master branches exist. Which should be the rebase target?").
		Options(
			huh.NewOption("main", "main"),
			huh.NewOption("master", "master"),
		).
		Value(&selected).
		Run()
	return selected, err
}
