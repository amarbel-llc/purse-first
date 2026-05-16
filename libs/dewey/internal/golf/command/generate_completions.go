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
func (u *Utility) GenerateCompletions(dir string) error {
	if err := u.generateBashCompletion(dir); err != nil {
		return err
	}
	if err := u.generateZshCompletion(dir); err != nil {
		return err
	}
	return u.generateFishCompletion(dir)
}

type sortedCommand struct {
	name string
	cmd  *Command
}

func (u *Utility) sortedVisibleCommands() []sortedCommand {
	var cmds []sortedCommand
	for name, cmd := range u.VisibleCommands() {
		cmds = append(cmds, sortedCommand{name, cmd})
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].name < cmds[j].name
	})
	return cmds
}

func (u *Utility) generateBashCompletion(dir string) error {
	bashDir := filepath.Join(dir, "share", "bash-completion", "completions")
	if err := os.MkdirAll(bashDir, 0o755); err != nil {
		return err
	}

	cmds := u.sortedVisibleCommands()

	var b strings.Builder
	fmt.Fprintf(&b, "# bash completion for %s\n\n", u.Name)
	fmt.Fprintf(&b, "_%s() {\n", u.Name)
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
		var completableParams []OldParam
		// positionalCompletable: non-Bool params with Completer, in declaration
		// order. Mirrors the positional assignment logic in cli.go.
		var positionalCompletable []OldParam
		for _, p := range c.cmd.OldParams {
			flags = append(flags, "--"+p.Name)
			if p.Short != 0 {
				flags = append(flags, fmt.Sprintf("-%c", p.Short))
			}
			if p.Completer != nil {
				completableParams = append(completableParams, p)
				if p.Type != Bool {
					positionalCompletable = append(positionalCompletable, p)
				}
			}
		}
		if len(flags) > 0 {
			fmt.Fprintf(&b, "        %s)\n", c.name)
			if len(completableParams) > 0 {
				fmt.Fprintf(&b, "            case \"${prev}\" in\n")
				for _, p := range completableParams {
					fmt.Fprintf(&b, "                --%s)\n", p.Name)
					fmt.Fprintf(&b, "                    COMPREPLY=( $(compgen -W \"$(%s __complete --command %s --param %s)\" -- \"${cur}\") )\n",
						u.Name, c.name, p.Name)
					fmt.Fprintf(&b, "                    ;;\n")
				}
				fmt.Fprintf(&b, "                *)\n")
				if len(positionalCompletable) > 0 {
					u.emitBashPositionalCompletions(&b, c.name, c.cmd.OldParams, positionalCompletable, flags)
				} else {
					fmt.Fprintf(&b, "                    COMPREPLY=( $(compgen -W %q -- \"${cur}\") )\n", strings.Join(flags, " "))
				}
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
	fmt.Fprintf(&b, "complete -F _%s %s\n", u.Name, u.Name)
	for _, alias := range u.Aliases {
		fmt.Fprintf(&b, "complete -F _%s %s\n", u.Name, alias)
	}

	content := []byte(b.String())
	if err := os.WriteFile(filepath.Join(bashDir, u.Name), content, 0o644); err != nil {
		return err
	}
	for _, alias := range u.Aliases {
		if err := os.WriteFile(filepath.Join(bashDir, alias), content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (u *Utility) generateZshCompletion(dir string) error {
	zshDir := filepath.Join(dir, "share", "zsh", "site-functions")
	if err := os.MkdirAll(zshDir, 0o755); err != nil {
		return err
	}

	cmds := u.sortedVisibleCommands()

	var b strings.Builder
	fmt.Fprintf(&b, "#compdef %s\n\n", u.Name)
	fmt.Fprintf(&b, "_%s() {\n", u.Name)
	fmt.Fprintf(&b, "    local -a commands\n")
	fmt.Fprintf(&b, "    commands=(\n")
	for _, c := range cmds {
		desc := strings.ReplaceAll(c.cmd.Description.Short, "'", "'\\''")
		fmt.Fprintf(&b, "        '%s:%s'\n", c.name, desc)
	}
	fmt.Fprintf(&b, "    )\n\n")
	fmt.Fprintf(&b, "    _describe 'command' commands\n")
	fmt.Fprintf(&b, "}\n\n")
	fmt.Fprintf(&b, "_%s\n", u.Name)
	for _, alias := range u.Aliases {
		fmt.Fprintf(&b, "compdef _%s %s\n", u.Name, alias)
	}

	content := []byte(b.String())
	if err := os.WriteFile(filepath.Join(zshDir, "_"+u.Name), content, 0o644); err != nil {
		return err
	}
	for _, alias := range u.Aliases {
		if err := os.WriteFile(filepath.Join(zshDir, "_"+alias), content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (u *Utility) generateFishCompletion(dir string) error {
	fishDir := filepath.Join(dir, "share", "fish", "vendor_completions.d")
	if err := os.MkdirAll(fishDir, 0o755); err != nil {
		return err
	}

	cmds := u.sortedVisibleCommands()

	var b strings.Builder
	fmt.Fprintf(&b, "# fish completion for %s\n\n", u.Name)
	fmt.Fprintf(&b, "complete -c %s -f\n\n", u.Name)

	for _, c := range cmds {
		desc := strings.ReplaceAll(c.cmd.Description.Short, "'", "\\'")
		fmt.Fprintf(&b, "complete -c %s -n '__fish_use_subcommand' -a %s -d '%s'\n",
			u.Name, c.name, desc)
	}

	u.emitFishParamCompletions(&b, u.Name, cmds)

	if err := os.WriteFile(filepath.Join(fishDir, u.Name+".fish"), []byte(b.String()), 0o644); err != nil {
		return err
	}

	for _, alias := range u.Aliases {
		var ab strings.Builder
		fmt.Fprintf(&ab, "# fish completion for %s (alias of %s)\n\n", alias, u.Name)
		fmt.Fprintf(&ab, "complete -c %s -f\n\n", alias)

		for _, c := range cmds {
			desc := strings.ReplaceAll(c.cmd.Description.Short, "'", "\\'")
			fmt.Fprintf(&ab, "complete -c %s -n '__fish_use_subcommand' -a %s -d '%s'\n",
				alias, c.name, desc)
		}

		u.emitFishParamCompletions(&ab, alias, cmds)

		if err := os.WriteFile(filepath.Join(fishDir, alias+".fish"), []byte(ab.String()), 0o644); err != nil {
			return err
		}
	}

	return nil
}

// emitBashPositionalCompletions writes bash completion logic that counts
// positional args typed so far and calls __complete for the corresponding
// positional param. This mirrors the positional assignment logic in cli.go:
// non-flag args are assigned to non-Bool params in declaration order.
func (u *Utility) emitBashPositionalCompletions(
	b *strings.Builder,
	cmdName string,
	allParams []OldParam,
	positionalCompletable []OldParam,
	flags []string,
) {
	// Build a set of all non-Bool params for positional index counting.
	// We need all non-Bool params (not just completable ones) to correctly
	// compute which positional slot the cursor is at.
	var allNonBool []OldParam
	for _, p := range allParams {
		if p.Type != Bool {
			allNonBool = append(allNonBool, p)
		}
	}

	// Count positional args already typed (words that aren't flags and
	// aren't values consumed by a preceding non-Bool flag).
	fmt.Fprintf(b, "                    local _pos=0 _i\n")
	fmt.Fprintf(b, "                    for (( _i=2; _i < COMP_CWORD; _i++ )); do\n")
	fmt.Fprintf(b, "                        case \"${COMP_WORDS[_i]}\" in\n")
	fmt.Fprintf(b, "                            -*) ;;\n")
	fmt.Fprintf(b, "                            *)\n")
	fmt.Fprintf(b, "                                case \"${COMP_WORDS[_i-1]}\" in\n")
	for _, p := range allNonBool {
		fmt.Fprintf(b, "                                    --%s) ;;\n", p.Name)
	}
	fmt.Fprintf(b, "                                    *) (( _pos++ )) ;;\n")
	fmt.Fprintf(b, "                                esac\n")
	fmt.Fprintf(b, "                                ;;\n")
	fmt.Fprintf(b, "                        esac\n")
	fmt.Fprintf(b, "                    done\n")

	// Map positional completable params to their positional indices
	// among all non-Bool params.
	type posEntry struct {
		index int
		param OldParam
	}
	var entries []posEntry
	for posIdx, nbp := range allNonBool {
		for _, pc := range positionalCompletable {
			if nbp.Name == pc.Name {
				entries = append(entries, posEntry{posIdx, pc})
				break
			}
		}
	}

	fmt.Fprintf(b, "                    case \"${_pos}\" in\n")
	for _, e := range entries {
		fmt.Fprintf(b, "                        %d)\n", e.index)
		fmt.Fprintf(b, "                            COMPREPLY=( $(compgen -W \"$(%s __complete --command %s --param %s)\" -- \"${cur}\") )\n",
			u.Name, cmdName, e.param.Name)
		fmt.Fprintf(b, "                            ;;\n")
	}
	fmt.Fprintf(b, "                        *)\n")
	fmt.Fprintf(b, "                            COMPREPLY=( $(compgen -W %q -- \"${cur}\") )\n", strings.Join(flags, " "))
	fmt.Fprintf(b, "                            ;;\n")
	fmt.Fprintf(b, "                    esac\n")
}

// emitFishParamCompletions writes fish completion rules for all params of all
// commands, including positional completion rules for non-Bool params with
// Completer. cmdName is the command name to use in `complete -c` (the primary
// binary name or an alias).
func (u *Utility) emitFishParamCompletions(b *strings.Builder, cmdName string, cmds []sortedCommand) {
	for _, c := range cmds {
		if c.cmd.PassthroughArgs {
			continue
		}
		for _, p := range c.cmd.OldParams {
			desc := strings.ReplaceAll(p.Description, "'", "\\'")
			shortOpt := ""
			if p.Short != 0 {
				shortOpt = fmt.Sprintf(" -s %c", p.Short)
			}
			completerArg := ""
			if p.Completer != nil {
				completerArg = fmt.Sprintf(" -ra '(%s __complete --command %s --param %s)'",
					u.Name, c.name, p.Name)
			}
			fmt.Fprintf(b, "complete -c %s -n '__fish_seen_subcommand_from %s' -l %s%s -d '%s'%s\n",
				cmdName, c.name, p.Name, shortOpt, desc, completerArg)
		}
		// Positional completions: for non-Bool params with Completer, add a
		// rule that fires when the flag form hasn't been used. This allows
		// positional arg completion (e.g., `cmd 42<TAB>` instead of requiring
		// `cmd --pr 42<TAB>`).
		for _, p := range c.cmd.OldParams {
			if p.Type == Bool || p.Completer == nil {
				continue
			}
			desc := strings.ReplaceAll(p.Description, "'", "\\'")
			fmt.Fprintf(b, "complete -c %s -n '__fish_seen_subcommand_from %s; and not __fish_contains_opt %s' -ra '(%s __complete --command %s --param %s)' -d '%s'\n",
				cmdName, c.name, p.Name, u.Name, c.name, p.Name, desc)
		}
	}
}
