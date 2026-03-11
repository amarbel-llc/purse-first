package packagebrew

import (
	"strings"
	"testing"
)

func TestGenerateReadme(t *testing.T) {
	readme := GenerateReadme(ReadmeOptions{
		TapName:     "org/tap",
		Description: "My marketplace",
		Packages: []ReadmePackage{
			{Name: "grit", Description: "Git MCP"},
			{Name: "bob", Description: "Skills"},
		},
		MetaFormulaName: "my-marketplace",
	})

	if !strings.Contains(readme, "brew tap org/tap") {
		t.Error("missing tap instruction")
	}
	if !strings.Contains(readme, "grit") {
		t.Error("missing grit package")
	}
	if !strings.Contains(readme, "brew install my-marketplace") {
		t.Error("missing meta install instruction")
	}
}
