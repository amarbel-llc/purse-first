package validate

import (
	"testing"

	"github.com/amarbel-llc/spinclass/internal/sweatfile"
)

func TestCheckClaudeAllowSyntaxValid(t *testing.T) {
	sf := sweatfile.Sweatfile{
		ClaudeAllow: []string{"Read", "Bash(git *)", "Write(/foo/*)"},
	}
	issues := CheckClaudeAllow(sf)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestCheckClaudeAllowSyntaxInvalid(t *testing.T) {
	sf := sweatfile.Sweatfile{
		ClaudeAllow: []string{"Bash(git *", "Read("},
	}
	issues := CheckClaudeAllow(sf)
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %v", issues)
	}
	for _, iss := range issues {
		if iss.Severity != SeverityError {
			t.Errorf("expected error severity, got %s", iss.Severity)
		}
	}
}

func TestCheckClaudeAllowMCPTool(t *testing.T) {
	sf := sweatfile.Sweatfile{
		ClaudeAllow: []string{
			"mcp__plugin_lux_lux__diagnostics",
			"mcp__plugin_grit_grit__status",
			"mcp__foo",
		},
	}
	issues := CheckClaudeAllow(sf)
	if len(issues) != 0 {
		t.Errorf("expected no issues for MCP tool names, got %v", issues)
	}
}

func TestCheckClaudeAllowUnknownTool(t *testing.T) {
	sf := sweatfile.Sweatfile{
		ClaudeAllow: []string{"FooBar", "Read"},
	}
	issues := CheckClaudeAllow(sf)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %v", issues)
	}
	if issues[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %s", issues[0].Severity)
	}
}

func TestCheckGitExcludesValid(t *testing.T) {
	sf := sweatfile.Sweatfile{
		GitSkipIndex: []string{".claude/", ".direnv/"},
	}
	issues := CheckGitExcludes(sf)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestCheckGitExcludesEmpty(t *testing.T) {
	sf := sweatfile.Sweatfile{
		GitSkipIndex: []string{".claude/", "", ".direnv/"},
	}
	issues := CheckGitExcludes(sf)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %v", issues)
	}
}

func TestCheckGitExcludesAbsolutePath(t *testing.T) {
	sf := sweatfile.Sweatfile{
		GitSkipIndex: []string{"/absolute/path"},
	}
	issues := CheckGitExcludes(sf)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %v", issues)
	}
}

func TestCheckMergedDuplicates(t *testing.T) {
	sf := sweatfile.Sweatfile{
		GitSkipIndex: []string{".claude/", ".direnv/", ".claude/"},
		ClaudeAllow: []string{"Read", "Read"},
	}
	issues := CheckMerged(sf)
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues (one per field), got %v", issues)
	}
	for _, iss := range issues {
		if iss.Severity != SeverityWarning {
			t.Errorf("expected warning severity, got %s", iss.Severity)
		}
	}
}

func TestCheckUnknownFields(t *testing.T) {
	data := []byte(`
git-excludes = [".claude/"]
unknown_field = "bad"
`)
	issues := CheckUnknownFields(data)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %v", issues)
	}
	if issues[0].Severity != SeverityError {
		t.Errorf("expected error severity, got %s", issues[0].Severity)
	}
}

func TestCheckUnknownFieldsClean(t *testing.T) {
	data := []byte(`
git-excludes = [".claude/"]
claude-allow = ["Read"]
`)
	issues := CheckUnknownFields(data)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestCheckUnknownFieldsHooksTable(t *testing.T) {
	data := []byte(`
[hooks]
create = "npm install"
stop = "just build test"
`)
	issues := CheckUnknownFields(data)
	if len(issues) != 0 {
		t.Errorf("expected no issues for [hooks] table, got %v", issues)
	}
}
