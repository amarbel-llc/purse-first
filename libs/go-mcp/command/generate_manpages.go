package command

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GenerateManpages writes roff-formatted manpages to {dir}/share/man/man1/.
// One page per app ({name}.1) and one per non-hidden command ({name}-{cmd}.1).
func (a *App) GenerateManpages(dir string) error {
	manDir := filepath.Join(dir, "share", "man", "man1")
	if err := os.MkdirAll(manDir, 0o755); err != nil {
		return err
	}

	if err := a.writeAppManpage(manDir); err != nil {
		return err
	}

	for name, cmd := range a.AllCommands() {
		if cmd.Hidden {
			continue
		}
		if err := a.writeCommandManpage(manDir, name, cmd); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) writeAppManpage(dir string) error {
	var b strings.Builder
	date := time.Now().Format("2006-01-02")
	name := strings.ToUpper(a.Name)

	fmt.Fprintf(&b, ".TH %s 1 %q %q\n", name, date, a.Name+" "+a.Version)
	fmt.Fprintf(&b, ".SH NAME\n")
	fmt.Fprintf(&b, "%s \\- %s\n", a.Name, a.Description.Short)

	// SYNOPSIS
	fmt.Fprintf(&b, ".SH SYNOPSIS\n")
	fmt.Fprintf(&b, ".B %s\n", a.Name)
	fmt.Fprintf(&b, ".I command\n")
	fmt.Fprintf(&b, ".RI [ options ]\n")

	if a.Description.Long != "" {
		fmt.Fprintf(&b, ".SH DESCRIPTION\n")
		fmt.Fprintf(&b, "%s\n", a.Description.Long)
	}

	type namedCmd struct {
		name string
		cmd  *Command
	}
	var cmds []namedCmd
	for cmdName, cmd := range a.VisibleCommands() {
		cmds = append(cmds, namedCmd{cmdName, cmd})
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].name < cmds[j].name
	})

	if len(cmds) > 0 {
		fmt.Fprintf(&b, ".SH COMMANDS\n")
		for _, nc := range cmds {
			fmt.Fprintf(&b, ".TP\n")
			fmt.Fprintf(&b, ".B %s\n", nc.name)
			fmt.Fprintf(&b, "%s\n", nc.cmd.Description.Short)
		}
	}

	writeExamples(&b, a.Examples)

	path := filepath.Join(dir, a.Name+".1")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func (a *App) writeCommandManpage(dir string, registeredName string, cmd *Command) error {
	var b strings.Builder
	date := time.Now().Format("2006-01-02")
	fullName := a.Name + "-" + registeredName
	upperName := strings.ToUpper(fullName)

	fmt.Fprintf(&b, ".TH %s 1 %q %q\n", upperName, date, a.Name+" "+a.Version)
	fmt.Fprintf(&b, ".SH NAME\n")
	fmt.Fprintf(&b, "%s \\- %s\n", fullName, cmd.Description.Short)

	// SYNOPSIS
	fmt.Fprintf(&b, ".SH SYNOPSIS\n")
	fmt.Fprintf(&b, ".B %s %s\n", a.Name, registeredName)
	for _, p := range cmd.Params {
		if p.Required {
			fmt.Fprintf(&b, ".RI --%s = %s\n", p.Name, strings.ToUpper(p.Type.JSONSchemaType()))
		} else {
			fmt.Fprintf(&b, ".RI [ --%s = %s ]\n", p.Name, strings.ToUpper(p.Type.JSONSchemaType()))
		}
	}

	desc := cmd.Description.Long
	if desc == "" {
		desc = cmd.Description.Short
	}
	fmt.Fprintf(&b, ".SH DESCRIPTION\n")
	fmt.Fprintf(&b, "%s\n", desc)

	if len(cmd.Params) > 0 {
		fmt.Fprintf(&b, ".SH OPTIONS\n")
		for _, p := range cmd.Params {
			fmt.Fprintf(&b, ".TP\n")
			label := fmt.Sprintf("--%s", p.Name)
			if p.Required {
				label += " (required)"
			}
			fmt.Fprintf(&b, ".B %s\n", label)
			fmt.Fprintf(&b, "%s\n", p.Description)
			if p.Default != nil {
				fmt.Fprintf(&b, "Default: %v\n", p.Default)
			}
		}
	}

	if len(cmd.Aliases) > 0 {
		fmt.Fprintf(&b, ".SH ALIASES\n")
		fmt.Fprintf(&b, "%s\n", strings.Join(cmd.Aliases, ", "))
	}

	writeExamples(&b, cmd.Examples)

	fmt.Fprintf(&b, ".SH SEE ALSO\n")
	fmt.Fprintf(&b, ".BR %s (1)\n", a.Name)

	path := filepath.Join(dir, fullName+".1")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeExamples(b *strings.Builder, examples []Example) {
	if len(examples) == 0 {
		return
	}
	fmt.Fprintf(b, ".SH EXAMPLES\n")
	for _, ex := range examples {
		fmt.Fprintf(b, ".TP\n")
		fmt.Fprintf(b, "%s\n", ex.Description)
		fmt.Fprintf(b, ".nf\n")
		for _, line := range strings.Split(ex.Command, "\n") {
			fmt.Fprintf(b, "$ %s\n", line)
		}
		if ex.Output != "" {
			fmt.Fprintf(b, "%s\n", ex.Output)
		}
		fmt.Fprintf(b, ".fi\n")
	}
}
