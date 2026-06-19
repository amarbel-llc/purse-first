package dagnabit_rust

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// patternPair is one ast-grep pattern/rewrite rule.
type patternPair struct {
	pattern string
	rewrite string
}

// renamePatterns is the curated ast-grep pattern set for a crate
// rename. oldLib/newLib are lib-target names (underscored). The set is
// validated by fixture tests, not by enumeration: if cargo check fails
// after a rewrite, the set is incomplete — extend it here.
//
// A single bare-identifier pattern covers everything. Empirically (ast-grep
// 0.42.1, tree-sitter rust): the identifier pattern matches the whole
// identifier AST node in every reference context — `use x::a;`,
// `use x::{a, b};`, `use x;`, `extern crate x;`, qualified expressions
// (`x::f()`), and type positions (`-> x::T`, struct fields,
// `impl From<x::T>`) — with no substring false positives (`other_x`,
// `x_thing` are distinct identifier nodes and never match). The
// previously sketched `x::$$$REST` path pattern is parsed in expression
// context and provably misses brace use-trees and all type positions.
func renamePatterns(oldLib, newLib string) []patternPair {
	return []patternPair{
		{pattern: oldLib, rewrite: newLib},
	}
}

// runAstGrep applies one pattern pair under dir. dryRun counts matches
// without writing: `--json=compact` replaces `--update-all` (the two
// are mutually exclusive in ast-grep — JSON output suppresses the
// update) and the match count is the length of the emitted array.
// The non-dry-run write path returns 0 matches; callers wanting counts
// run a dry pass first. Counts are therefore advisory: nothing locks
// the tree between the count pass and the write pass, so a concurrent
// edit can make them diverge.
//
// ast-grep run exits 1 when it finds no matches (grep semantics), so a
// dry-run exit error whose stdout still parses as a JSON match array is
// a successful zero-match scan, not a failure.
func runAstGrep(dir string, p patternPair, dryRun bool) (matches int, err error) {
	args := []string{
		"run",
		"--lang", "rust",
		"--pattern", p.pattern,
		"--rewrite", p.rewrite,
	}

	if dryRun {
		args = append(args, "--json=compact")
	} else {
		args = append(args, "--update-all")
	}

	args = append(args, dir)

	cmd := exec.Command("ast-grep", args...)

	out, runErr := cmd.Output()

	if dryRun {
		var matchObjects []json.RawMessage

		if jsonErr := json.Unmarshal(out, &matchObjects); jsonErr == nil {
			return len(matchObjects), nil
		}
	}

	if runErr != nil {
		var exitErr *exec.ExitError
		var stderr []byte

		if errors.As(runErr, &exitErr) {
			stderr = exitErr.Stderr
		}

		return 0, fmt.Errorf(
			"ast-grep %s: %w: %s",
			strings.Join(args, " "),
			runErr,
			stderr,
		)
	}

	if dryRun {
		return 0, fmt.Errorf("decoding ast-grep --json output for %s", dir)
	}

	return 0, nil
}
