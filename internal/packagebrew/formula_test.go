package packagebrew

import (
	"strings"
	"testing"
)

func TestGenerateBinaryFormula(t *testing.T) {
	formula := GenerateFormula(FormulaOptions{
		Name:        "grit",
		Description: "Git operations via MCP",
		Version:     "1.0.0",
		Homepage:    "https://example.com",
		License:     "MIT",
		ReleaseRepo: "org/homebrew-tap",
		Binary:      true,
		Hashes: map[string]string{
			"darwin-arm64": "abc123",
			"linux-amd64":  "def456",
		},
		BrewDeps: []string{"gh"},
	})

	if !strings.Contains(formula, "class Grit < Formula") {
		t.Error("missing PascalCase class name")
	}
	if !strings.Contains(formula, `desc "Git operations via MCP"`) {
		t.Error("missing description")
	}
	if !strings.Contains(formula, `sha256 "abc123"`) {
		t.Error("missing darwin-arm64 hash")
	}
	if !strings.Contains(formula, `sha256 "def456"`) {
		t.Error("missing linux-amd64 hash")
	}
	if !strings.Contains(formula, `bin.install "grit"`) {
		t.Error("missing bin.install for binary package")
	}
	if !strings.Contains(formula, `depends_on "gh"`) {
		t.Error("missing dependency")
	}
	if !strings.Contains(formula, `system bin/"grit", "--help"`) {
		t.Error("missing binary test block")
	}
}

func TestGenerateSkillOnlyFormula(t *testing.T) {
	formula := GenerateFormula(FormulaOptions{
		Name:        "bob",
		Description: "Skills package",
		Version:     "0.1.0",
		License:     "MIT",
		ReleaseRepo: "org/homebrew-tap",
		Binary:      false,
		Hashes: map[string]string{
			"": "xyz789",
		},
	})

	if !strings.Contains(formula, "class Bob < Formula") {
		t.Error("missing class name")
	}
	if strings.Contains(formula, "bin.install") {
		t.Error("skill-only formula should not have bin.install")
	}
	if strings.Contains(formula, "on_macos") {
		t.Error("skill-only formula should not have platform blocks")
	}
	if !strings.Contains(formula, "assert_predicate") {
		t.Error("missing assert_predicate test for skill-only")
	}
}

func TestGenerateMetaFormula(t *testing.T) {
	formula := GenerateMetaFormula(MetaFormulaOptions{
		Name:        "my-marketplace",
		Description: "All packages",
		Version:     "1.0.0",
		License:     "MIT",
		ReleaseRepo: "org/homebrew-tap",
		Hash:        "meta123",
		Packages:    []string{"grit", "bob"},
		AutoInstall: true,
	})

	if !strings.Contains(formula, "class MyMarketplace < Formula") {
		t.Error("missing PascalCase class name")
	}
	if !strings.Contains(formula, `depends_on "grit"`) {
		t.Error("missing grit dependency")
	}
	if !strings.Contains(formula, `depends_on "bob"`) {
		t.Error("missing bob dependency")
	}
	if !strings.Contains(formula, `system "purse-first", "install"`) {
		t.Error("missing post_install auto-install")
	}
}

func TestGenerateMetaFormulaNoAutoInstall(t *testing.T) {
	formula := GenerateMetaFormula(MetaFormulaOptions{
		Name:        "my-marketplace",
		Description: "All packages",
		Version:     "1.0.0",
		License:     "MIT",
		ReleaseRepo: "org/homebrew-tap",
		Hash:        "meta123",
		Packages:    []string{"grit"},
		AutoInstall: false,
	})

	if strings.Contains(formula, "post_install") {
		t.Error("should not have post_install when AutoInstall is false")
	}
}

func TestToClassName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"grit", "Grit"},
		{"get-hubbed", "GetHubbed"},
		{"purse-first", "PurseFirst"},
		{"tap-dancer", "TapDancer"},
		{"purse-first-all", "PurseFirstAll"},
	}
	for _, tt := range tests {
		got := toClassName(tt.input)
		if got != tt.want {
			t.Errorf("toClassName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
