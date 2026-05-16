package command

import "strings"

// normalizeGitCommand strips global git options (-C <path>, --no-pager,
// -c key=val, --git-dir, --work-tree, --bare) from between "git" and
// the subcommand. This lets prefix-based hook matching work regardless of
// how the agent invokes git. Non-git commands are returned unchanged.
func normalizeGitCommand(cmd string) string {
	tokens := strings.Fields(cmd)
	if len(tokens) == 0 || tokens[0] != "git" {
		return cmd
	}

	var kept []string
	i := 1
	for i < len(tokens) {
		tok := tokens[i]

		// --flag=value forms for two-token options
		if strings.HasPrefix(tok, "-C=") ||
			strings.HasPrefix(tok, "-c=") ||
			strings.HasPrefix(tok, "--git-dir=") ||
			strings.HasPrefix(tok, "--work-tree=") {
			i++
			continue
		}

		// two-token options: flag + value
		if tok == "-C" || tok == "-c" || tok == "--git-dir" || tok == "--work-tree" {
			i += 2 // skip flag and its argument
			continue
		}

		// single-token flags
		if tok == "--no-pager" || tok == "--bare" {
			i++
			continue
		}

		// first non-option token is the subcommand; stop stripping
		break
	}

	kept = append(kept, "git")
	kept = append(kept, tokens[i:]...)

	return strings.Join(kept, " ")
}
