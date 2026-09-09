package command

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// maxManNameDescription is the longest NAME-line description the fleet manpage
// index renders as a single row. doppelgang's lint-man enforces the same budget
// over rendered pages, so emitting past it only defers the failure to the gate.
const maxManNameDescription = 72

// manNameDescription picks the text following "name \- " on a page's NAME line.
//
// Description.Short is used verbatim whenever it can serve, so every page that
// already satisfies the contract keeps rendering identically. Short is also the
// MCP tool description though, and those run to paragraphs — so when it cannot
// serve, Title is tried as an explicit override and then Short's opening
// clause. MCP registration is untouched by any of this: it reads
// Description.Short directly, and the full text still reaches the page body via
// DESCRIPTION.
func manNameDescription(page, short, title string) (string, error) {
	for _, candidate := range []string{short, title, firstManNameClause(short)} {
		if isManNameDescription(candidate) {
			return candidate, nil
		}
	}

	best := firstManNameClause(short)
	if best == "" {
		best = firstManNameClause(title)
	}
	if best == "" {
		return "", fmt.Errorf(
			"man page %s: NAME description is empty; set Description.Short or Title",
			page,
		)
	}
	return "", fmt.Errorf(
		"man page %s: NAME description is %d chars, max %d: %q; set Title to a "+
			"one-line summary (Description.Short stays the MCP tool description)",
		page, utf8.RuneCountInString(best), maxManNameDescription, best,
	)
}

func isManNameDescription(s string) bool {
	return s != "" &&
		!strings.ContainsAny(s, "\n\r") &&
		utf8.RuneCountInString(s) <= maxManNameDescription
}

// firstManNameClause returns the leading clause of s: everything before the
// first line break, or before the first '.', '!' or '?' that ends the string or
// is followed by whitespace, so "and/or" and "feed/{id}" survive. Abbreviations
// such as "e.g." are not special-cased — keep a description's opening clause
// free of them, or set Title.
func firstManNameClause(s string) string {
	s = strings.TrimSpace(s)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n', '\r':
			return strings.TrimSpace(s[:i])
		case '.', '!', '?':
			if i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' || s[i+1] == '\n' {
				return strings.TrimSpace(s[:i])
			}
		}
	}
	return s
}

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

	nameDesc, err := manNameDescription(a.Name, a.Description.Short, "")
	if err != nil {
		return err
	}

	fmt.Fprintf(&b, ".TH %s 1 %q %q\n", name, date, a.Name+" "+a.Version)
	fmt.Fprintf(&b, ".SH NAME\n")
	fmt.Fprintf(&b, "%s \\- %s\n", a.Name, nameDesc)

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
			// The COMMANDS list is the same one-line-summary role as a NAME
			// line, so it takes the same derivation rather than a paragraph.
			summary, err := manNameDescription(
				a.Name+"-"+nc.name, nc.cmd.Description.Short, nc.cmd.Title,
			)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, ".TP\n")
			fmt.Fprintf(&b, ".BR %s (1)\n", nc.name)
			fmt.Fprintf(&b, "%s\n", summary)
		}
	}

	writeExamples(&b, a.Examples)
	writeEnvironment(&b, a.EnvVars)
	writeFiles(&b, a.Files)

	if len(cmds) > 0 {
		fmt.Fprintf(&b, ".SH SEE ALSO\n")
		var refs []string
		for _, nc := range cmds {
			refs = append(refs, fmt.Sprintf(".BR %s-%s (1)", a.Name, nc.name))
		}
		fmt.Fprintf(&b, "%s\n", strings.Join(refs, ",\n"))
	}

	path := filepath.Join(dir, a.Name+".1")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func (a *App) writeCommandManpage(dir string, registeredName string, cmd *Command) error {
	var b strings.Builder
	date := time.Now().Format("2006-01-02")
	fullName := a.Name + "-" + registeredName
	upperName := strings.ToUpper(fullName)

	nameDesc, err := manNameDescription(fullName, cmd.Description.Short, cmd.Title)
	if err != nil {
		return err
	}

	fmt.Fprintf(&b, ".TH %s 1 %q %q\n", upperName, date, a.Name+" "+a.Version)
	fmt.Fprintf(&b, ".SH NAME\n")
	fmt.Fprintf(&b, "%s \\- %s\n", fullName, nameDesc)

	// SYNOPSIS
	fmt.Fprintf(&b, ".SH SYNOPSIS\n")
	fmt.Fprintf(&b, ".B %s %s\n", a.Name, registeredName)
	if cmd.PassthroughArgs {
		fmt.Fprintf(&b, ".RI [ args... ]\n")
	} else {
		for _, p := range cmd.Params {
			// A variadic param is positional and repeatable, so render it
			// as ARG... rather than as a --flag=TYPE it is never given as.
			if p.Variadic {
				arg := strings.ToUpper(p.Name) + "..."
				if p.Required {
					// .I, not .RI: .RI alternates roman/italic starting
					// roman, so a lone argument would typeset roman — the
					// opposite of a placeholder. The optional form below
					// works because the brackets take the roman slots.
					fmt.Fprintf(&b, ".I %s\n", arg)
				} else {
					fmt.Fprintf(&b, ".RI [ %s ]\n", arg)
				}
				continue
			}
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

	if len(cmd.Params) > 0 && !cmd.PassthroughArgs {
		fmt.Fprintf(&b, ".SH OPTIONS\n")
		for _, p := range cmd.Params {
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
	seeAlsoRefs = append(seeAlsoRefs, fmt.Sprintf(".BR %s (1)", a.Name))
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
