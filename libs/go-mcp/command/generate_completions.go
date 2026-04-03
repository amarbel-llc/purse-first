package command

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GenerateCompletions writes shell completion scripts for bash, zsh, and fish
// to standard paths under {dir}/share/.
func (a *App) GenerateCompletions(dir string) error {
	if err := a.generateBashCompletion(dir); err != nil {
		return err
	}
	if err := a.generateZshCompletion(dir); err != nil {
		return err
	}
	return a.generateFishCompletion(dir)
}

type sortedCommand struct {
	name string
	cmd  *Command
}

func (a *App) sortedVisibleCommands() []sortedCommand {
	var cmds []sortedCommand
	for name, cmd := range a.VisibleCommands() {
		cmds = append(cmds, sortedCommand{name, cmd})
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].name < cmds[j].name
	})
	return cmds
}

func (a *App) generateBashCompletion(dir string) error {
	bashDir := filepath.Join(dir, "share", "bash-completion", "completions")
	if err := os.MkdirAll(bashDir, 0o755); err != nil {
		return err
	}

	cmds := a.sortedVisibleCommands()

	var b strings.Builder
	fmt.Fprintf(&b, "# bash completion for %s\n\n", a.Name)
	fmt.Fprintf(&b, "_%s() {\n", a.Name)
	fmt.Fprintf(&b, "    local cur prev commands\n")
	fmt.Fprintf(&b, "    COMPREPLY=()\n")
	fmt.Fprintf(&b, "    cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	fmt.Fprintf(&b, "    prev=\"${COMP_WORDS[COMP_CWORD-1]}\"\n\n")

	var names []string
	for _, c := range cmds {
		names = append(names, c.name)
	}
	fmt.Fprintf(&b, "    commands=%q\n\n", strings.Join(names, " "))

	fmt.Fprintf(&b, "    if [[ ${COMP_CWORD} -eq 1 ]]; then\n")
	fmt.Fprintf(&b, "        COMPREPLY=( $(compgen -W \"${commands}\" -- \"${cur}\") )\n")
	fmt.Fprintf(&b, "        return 0\n")
	fmt.Fprintf(&b, "    fi\n\n")

	fmt.Fprintf(&b, "    local subcmd=\"${COMP_WORDS[1]}\"\n")
	fmt.Fprintf(&b, "    case \"${subcmd}\" in\n")
	for _, c := range cmds {
		if c.cmd.PassthroughArgs {
			continue
		}
		var flags []string
		var completableParams []Param
		for _, p := range c.cmd.Params {
			flags = append(flags, "--"+p.Name)
			if p.Short != 0 {
				flags = append(flags, fmt.Sprintf("-%c", p.Short))
			}
			if p.Completer != nil {
				completableParams = append(completableParams, p)
			}
		}
		if len(flags) > 0 {
			fmt.Fprintf(&b, "        %s)\n", c.name)
			if len(completableParams) > 0 {
				fmt.Fprintf(&b, "            case \"${prev}\" in\n")
				for _, p := range completableParams {
					fmt.Fprintf(&b, "                --%s)\n", p.Name)
					fmt.Fprintf(&b, "                    COMPREPLY=( $(compgen -W \"$(%s __complete --command %s --param %s)\" -- \"${cur}\") )\n",
						a.Name, c.name, p.Name)
					fmt.Fprintf(&b, "                    ;;\n")
				}
				fmt.Fprintf(&b, "                *)\n")
				fmt.Fprintf(&b, "                    COMPREPLY=( $(compgen -W %q -- \"${cur}\") )\n", strings.Join(flags, " "))
				fmt.Fprintf(&b, "                    ;;\n")
				fmt.Fprintf(&b, "            esac\n")
			} else {
				fmt.Fprintf(&b, "            COMPREPLY=( $(compgen -W %q -- \"${cur}\") )\n", strings.Join(flags, " "))
			}
			fmt.Fprintf(&b, "            ;;\n")
		}
	}
	fmt.Fprintf(&b, "    esac\n")
	fmt.Fprintf(&b, "}\n\n")
	fmt.Fprintf(&b, "complete -F _%s %s\n", a.Name, a.Name)

	return os.WriteFile(filepath.Join(bashDir, a.Name), []byte(b.String()), 0o644)
}

func (a *App) generateZshCompletion(dir string) error {
	zshDir := filepath.Join(dir, "share", "zsh", "site-functions")
	if err := os.MkdirAll(zshDir, 0o755); err != nil {
		return err
	}

	cmds := a.sortedVisibleCommands()

	var b strings.Builder
	fmt.Fprintf(&b, "#compdef %s\n\n", a.Name)
	fmt.Fprintf(&b, "_%s() {\n", a.Name)
	fmt.Fprintf(&b, "    local -a commands\n")
	fmt.Fprintf(&b, "    commands=(\n")
	for _, c := range cmds {
		desc := strings.ReplaceAll(c.cmd.Description.Short, "'", "'\\''")
		fmt.Fprintf(&b, "        '%s:%s'\n", c.name, desc)
	}
	fmt.Fprintf(&b, "    )\n\n")
	fmt.Fprintf(&b, "    _describe 'command' commands\n")
	fmt.Fprintf(&b, "}\n\n")
	fmt.Fprintf(&b, "_%s\n", a.Name)

	return os.WriteFile(filepath.Join(zshDir, "_"+a.Name), []byte(b.String()), 0o644)
}

func (a *App) generateFishCompletion(dir string) error {
	fishDir := filepath.Join(dir, "share", "fish", "vendor_completions.d")
	if err := os.MkdirAll(fishDir, 0o755); err != nil {
		return err
	}

	cmds := a.sortedVisibleCommands()

	var b strings.Builder
	fmt.Fprintf(&b, "# fish completion for %s\n\n", a.Name)
	fmt.Fprintf(&b, "complete -c %s -f\n\n", a.Name)

	for _, c := range cmds {
		desc := strings.ReplaceAll(c.cmd.Description.Short, "'", "\\'")
		fmt.Fprintf(&b, "complete -c %s -n '__fish_use_subcommand' -a %s -d '%s'\n",
			a.Name, c.name, desc)
	}

	for _, c := range cmds {
		if c.cmd.PassthroughArgs {
			continue
		}
		for _, p := range c.cmd.Params {
			desc := strings.ReplaceAll(p.Description, "'", "\\'")
			shortOpt := ""
			if p.Short != 0 {
				shortOpt = fmt.Sprintf(" -s %c", p.Short)
			}
			completerArg := ""
			if p.Completer != nil {
				completerArg = fmt.Sprintf(" -ra '(%s __complete --command %s --param %s)'",
					a.Name, c.name, p.Name)
			}
			fmt.Fprintf(&b, "complete -c %s -n '__fish_seen_subcommand_from %s' -l %s%s -d '%s'%s\n",
				a.Name, c.name, p.Name, shortOpt, desc, completerArg)
		}
	}

	return os.WriteFile(filepath.Join(fishDir, a.Name+".fish"), []byte(b.String()), 0o644)
}
