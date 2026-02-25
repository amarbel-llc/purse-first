package command

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// extractSimpleCommands parses a bash command string and returns the
// individual simple commands found in the AST. Handles &&, ||, ;, |,
// subshells, and strips redirections. On parse failure, returns the
// original command string as a single-element slice (fallback to raw
// prefix matching).
func extractSimpleCommands(command string) []string {
	if command == "" {
		return []string{""}
	}

	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(command), "")
	if err != nil {
		return []string{command}
	}

	var commands []string
	syntax.Walk(file, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}

		if len(call.Args) == 0 {
			return false
		}

		var parts []string
		printer := syntax.NewPrinter()
		for _, word := range call.Args {
			var sb strings.Builder
			printer.Print(&sb, word)
			parts = append(parts, sb.String())
		}

		commands = append(commands, strings.Join(parts, " "))
		return false
	})

	if len(commands) == 0 {
		return []string{command}
	}

	return commands
}
