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
func (u *Utility) GenerateManpages(dir string) error {
	manDir := filepath.Join(dir, "share", "man", "man1")
	if err := os.MkdirAll(manDir, 0o755); err != nil {
		return err
	}

	if err := u.writeAppManpage(manDir); err != nil {
		return err
	}

	for name, cmd := range u.AllCommands() {
		if cmd.Hidden {
			continue
		}
		if err := u.writeCommandManpage(manDir, name, cmd); err != nil {
			return err
		}
	}

	return nil
}

func (u *Utility) writeAppManpage(dir string) error {
	var b strings.Builder
	date := time.Now().Format("2006-01-02")
	name := strings.ToUpper(u.Name)

	fmt.Fprintf(&b, ".TH %s 1 %q %q\n", name, date, u.Name+" "+u.Version)
	fmt.Fprintf(&b, ".SH NAME\n")
	fmt.Fprintf(&b, "%s \\- %s\n", u.Name, u.Description.Short)

	// SYNOPSIS
	fmt.Fprintf(&b, ".SH SYNOPSIS\n")
	fmt.Fprintf(&b, ".B %s\n", u.Name)
	fmt.Fprintf(&b, ".I command\n")
	fmt.Fprintf(&b, ".RI [ options ]\n")

	if u.Description.Long != "" {
		fmt.Fprintf(&b, ".SH DESCRIPTION\n")
		fmt.Fprintf(&b, "%s\n", u.Description.Long)
	}

	type namedCmd struct {
		name string
		cmd  *Command
	}
	var cmds []namedCmd
	for cmdName, cmd := range u.VisibleCommands() {
		cmds = append(cmds, namedCmd{cmdName, cmd})
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].name < cmds[j].name
	})

	if len(cmds) > 0 {
		fmt.Fprintf(&b, ".SH COMMANDS\n")
		for _, nc := range cmds {
			fmt.Fprintf(&b, ".TP\n")
			fmt.Fprintf(&b, ".BR %s (1)\n", nc.name)
			fmt.Fprintf(&b, "%s\n", nc.cmd.Description.Short)
		}
	}

	writeExamples(&b, u.Examples)
	writeEnvironment(&b, u.EnvVars)
	writeFiles(&b, u.Files)

	if len(cmds) > 0 {
		fmt.Fprintf(&b, ".SH SEE ALSO\n")
		var refs []string
		for _, nc := range cmds {
			refs = append(refs, fmt.Sprintf(".BR %s-%s (1)", u.Name, nc.name))
		}
		fmt.Fprintf(&b, "%s\n", strings.Join(refs, ",\n"))
	}

	path := filepath.Join(dir, u.Name+".1")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func (u *Utility) writeCommandManpage(dir string, registeredName string, cmd *Command) error {
	var b strings.Builder
	date := time.Now().Format("2006-01-02")
	fullName := u.Name + "-" + registeredName
	upperName := strings.ToUpper(fullName)

	fmt.Fprintf(&b, ".TH %s 1 %q %q\n", upperName, date, u.Name+" "+u.Version)
	fmt.Fprintf(&b, ".SH NAME\n")
	fmt.Fprintf(&b, "%s \\- %s\n", fullName, cmd.Description.Short)

	// SYNOPSIS
	fmt.Fprintf(&b, ".SH SYNOPSIS\n")
	fmt.Fprintf(&b, ".B %s %s\n", u.Name, registeredName)
	if cmd.PassthroughArgs {
		fmt.Fprintf(&b, ".RI [ args... ]\n")
	} else {
		for _, p := range cmd.OldParams {
			flagStr := fmt.Sprintf("--%s", p.Name)
			if p.Short != 0 {
				flagStr = fmt.Sprintf("-%c | --%s", p.Short, p.Name)
			}
			if p.Required {
				fmt.Fprintf(&b, ".RI %s = %s\n", flagStr, strings.ToUpper(p.Type.JSONSchemaType()))
			} else {
				fmt.Fprintf(&b, ".RI [ %s = %s ]\n", flagStr, strings.ToUpper(p.Type.JSONSchemaType()))
			}
		}
	}

	desc := cmd.Description.Long
	if desc == "" {
		desc = cmd.Description.Short
	}
	fmt.Fprintf(&b, ".SH DESCRIPTION\n")
	fmt.Fprintf(&b, "%s\n", desc)

	if len(cmd.OldParams) > 0 && !cmd.PassthroughArgs {
		fmt.Fprintf(&b, ".SH OPTIONS\n")
		for _, p := range cmd.OldParams {
			fmt.Fprintf(&b, ".TP\n")
			label := fmt.Sprintf("--%s", p.Name)
			if p.Short != 0 {
				label = fmt.Sprintf("-%c, --%s", p.Short, p.Name)
			}
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
	writeEnvironment(&b, cmd.EnvVars)
	writeFiles(&b, cmd.Files)

	fmt.Fprintf(&b, ".SH SEE ALSO\n")
	var seeAlsoRefs []string
	seeAlsoRefs = append(seeAlsoRefs, fmt.Sprintf(".BR %s (1)", u.Name))
	for _, ref := range cmd.SeeAlso {
		seeAlsoRefs = append(seeAlsoRefs, fmt.Sprintf(".BR %s (1)", ref))
	}
	fmt.Fprintf(&b, "%s\n", strings.Join(seeAlsoRefs, ",\n"))

	path := filepath.Join(dir, fullName+".1")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// writeEnvironment renders an ENVIRONMENT section in man(7) format.
// Each EnvVar becomes a .TP entry with the variable name in bold,
// followed by its description and optional default.
func writeEnvironment(b *strings.Builder, vars []EnvVar) {
	if len(vars) == 0 {
		return
	}
	fmt.Fprintf(b, ".SH ENVIRONMENT\n")
	for _, v := range vars {
		fmt.Fprintf(b, ".TP\n")
		fmt.Fprintf(b, ".B %s\n", v.Name)
		if v.Description != "" {
			fmt.Fprintf(b, "%s\n", v.Description)
		}
		if v.Default != "" {
			fmt.Fprintf(b, "Default: %s\n", v.Default)
		}
	}
}

// writeFiles renders a FILES section in man(7) format. Each FilePath
// becomes a .TP entry with the path in italics, followed by its description.
func writeFiles(b *strings.Builder, files []FilePath) {
	if len(files) == 0 {
		return
	}
	fmt.Fprintf(b, ".SH FILES\n")
	for _, f := range files {
		fmt.Fprintf(b, ".TP\n")
		fmt.Fprintf(b, ".I %s\n", f.Path)
		if f.Description != "" {
			fmt.Fprintf(b, "%s\n", f.Description)
		}
	}
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
