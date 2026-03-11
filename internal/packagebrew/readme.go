package packagebrew

import (
	"fmt"
	"strings"
)

type ReadmeOptions struct {
	TapName         string
	Description     string
	Packages        []ReadmePackage
	MetaFormulaName string
}

type ReadmePackage struct {
	Name        string
	Description string
}

func GenerateReadme(opts ReadmeOptions) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s Homebrew Tap\n\n", opts.TapName)
	if opts.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", opts.Description)
	}

	b.WriteString("## Installation\n\n")
	fmt.Fprintf(&b, "```bash\nbrew tap %s\n```\n\n", opts.TapName)

	b.WriteString("## Available Packages\n\n")
	b.WriteString("| Package | Description |\n")
	b.WriteString("|---------|-------------|\n")
	for _, pkg := range opts.Packages {
		fmt.Fprintf(&b, "| `%s` | %s |\n", pkg.Name, pkg.Description)
	}
	fmt.Fprintf(&b, "| `%s` | Meta package — installs everything |\n\n", opts.MetaFormulaName)

	b.WriteString("## Install All\n\n")
	fmt.Fprintf(&b, "```bash\nbrew install %s\n```\n\n", opts.MetaFormulaName)

	b.WriteString("## Install Individual Packages\n\n")
	b.WriteString("```bash\n")
	for _, pkg := range opts.Packages {
		fmt.Fprintf(&b, "brew install %s\n", pkg.Name)
	}
	b.WriteString("```\n")

	return b.String()
}
