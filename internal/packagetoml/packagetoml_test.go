package packagetoml

import (
	"testing"
)

func TestParsePackageToml(t *testing.T) {
	input := []byte(`
name = "chix"
description = "Nix MCP server and skills for Claude Code"

[author]
name = "friedenberg"

[mcp.chix]
command = "chix"

[[hooks.PostToolUse]]
matcher = "Edit|Write"
command = "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix"
timeout = 30
`)

	pkg, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if pkg.Name != "chix" {
		t.Errorf("Name = %q, want %q", pkg.Name, "chix")
	}

	if pkg.Description != "Nix MCP server and skills for Claude Code" {
		t.Errorf("Description = %q, want %q", pkg.Description, "Nix MCP server and skills for Claude Code")
	}

	if pkg.Author.Name != "friedenberg" {
		t.Errorf("Author.Name = %q, want %q", pkg.Author.Name, "friedenberg")
	}

	if len(pkg.MCP) != 1 {
		t.Fatalf("len(MCP) = %d, want 1", len(pkg.MCP))
	}

	chixMCP, ok := pkg.MCP["chix"]
	if !ok {
		t.Fatal("MCP[\"chix\"] not found")
	}
	if chixMCP.Command != "chix" {
		t.Errorf("MCP[\"chix\"].Command = %q, want %q", chixMCP.Command, "chix")
	}

	if len(pkg.Hooks) != 1 {
		t.Fatalf("len(Hooks) = %d, want 1", len(pkg.Hooks))
	}

	postToolUse, ok := pkg.Hooks["PostToolUse"]
	if !ok {
		t.Fatal("Hooks[\"PostToolUse\"] not found")
	}
	if len(postToolUse) != 1 {
		t.Fatalf("len(PostToolUse) = %d, want 1", len(postToolUse))
	}

	hook := postToolUse[0]
	if hook.Matcher != "Edit|Write" {
		t.Errorf("hook.Matcher = %q, want %q", hook.Matcher, "Edit|Write")
	}
	if hook.Command != "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix" {
		t.Errorf("hook.Command = %q, want %q", hook.Command, "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix")
	}
	if hook.Timeout != 30 {
		t.Errorf("hook.Timeout = %d, want 30", hook.Timeout)
	}
}

func TestParseSkillOnlyPackage(t *testing.T) {
	input := []byte(`
name = "robin"
description = "Expert skill for setting up and writing BATS integration tests"

[author]
name = "friedenberg"
`)

	pkg, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if pkg.Name != "robin" {
		t.Errorf("Name = %q, want %q", pkg.Name, "robin")
	}

	if pkg.Description != "Expert skill for setting up and writing BATS integration tests" {
		t.Errorf("Description = %q, want %q", pkg.Description, "Expert skill for setting up and writing BATS integration tests")
	}

	if pkg.Author.Name != "friedenberg" {
		t.Errorf("Author.Name = %q, want %q", pkg.Author.Name, "friedenberg")
	}

	if len(pkg.MCP) != 0 {
		t.Errorf("len(MCP) = %d, want 0", len(pkg.MCP))
	}

	if len(pkg.Hooks) != 0 {
		t.Errorf("len(Hooks) = %d, want 0", len(pkg.Hooks))
	}
}
