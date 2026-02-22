package perms

import (
	"path/filepath"
)

const (
	ReviewPromoteGlobal = "global"
	ReviewPromoteRepo   = "repo"
	ReviewKeep          = "keep"
	ReviewDiscard       = "discard"
)

type ReviewDecision struct {
	Rule   string
	Action string
}

func RouteDecisions(tiersDir, repo, settingsPath string, decisions []ReviewDecision) error {
	var toRemove []string

	for _, d := range decisions {
		switch d.Action {
		case ReviewPromoteGlobal:
			globalPath := filepath.Join(tiersDir, "global.json")
			if err := AppendToTierFile(globalPath, d.Rule); err != nil {
				return err
			}
			toRemove = append(toRemove, d.Rule)

		case ReviewPromoteRepo:
			repoPath := filepath.Join(tiersDir, "repos", repo+".json")
			if err := AppendToTierFile(repoPath, d.Rule); err != nil {
				return err
			}
			toRemove = append(toRemove, d.Rule)

		case ReviewDiscard:
			toRemove = append(toRemove, d.Rule)

		case ReviewKeep:
			// Leave in settings, nothing to do.
		}
	}

	if len(toRemove) == 0 {
		return nil
	}

	current, err := LoadClaudeSettings(settingsPath)
	if err != nil {
		return err
	}

	remaining := RemoveRules(current, toRemove)

	return SaveClaudeSettings(settingsPath, remaining)
}
