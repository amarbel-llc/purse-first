// Package cargo_manifest performs comment-preserving, span-based edits
// on Cargo.toml files. Go TOML libraries do not round-trip comments, so
// mutation is line-oriented: parse just enough structure (section
// headers, key positions) to locate the edit, then rewrite the raw text
// span. Reads that need full TOML semantics use BurntSushi/toml
// elsewhere; this package never re-serializes whole documents.
package cargo_manifest

import (
	"fmt"
	"regexp"
	"strings"
)

// sectionRE matches both table headers ([dependencies]) and
// array-of-tables headers ([[bin]]); the latter must reset the current
// section so dependency edits never leak into a following [[bin]] /
// [[example]] block.
var sectionRE = regexp.MustCompile(`^\s*\[\[?([^\]]+)\]\]?\s*(?:#.*)?$`)

// dependencySectionBases are the dependency table names this package
// edits, in both their bare form ([dependencies]) and table form
// ([dependencies.<name>]). target.*.dependencies is intentionally out
// of scope.
var dependencySectionBases = []string{
	"dependencies",
	"dev-dependencies",
	"build-dependencies",
}

func isDependencySection(section string) bool {
	for _, base := range dependencySectionBases {
		if section == base || strings.HasPrefix(section, base+".") {
			return true
		}
	}

	return false
}

// forEachLine calls fn with (sectionName, line) and collects fn's
// replacement line. Section names are the literal header contents, e.g.
// "dependencies", "dependencies.foo", "workspace", "package". A header
// line is passed with its own (new) section name; if fn rewrites a
// header, the rewritten name governs subsequent lines.
//
// Invariant: fn may rename a section header but never remove one — the
// post-fn sectionRE re-check below depends on a header staying a header.
func forEachLine(manifest []byte, fn func(section, line string) string) []byte {
	lines := strings.Split(string(manifest), "\n")
	section := ""

	for i, line := range lines {
		if m := sectionRE.FindStringSubmatch(line); m != nil {
			section = m[1]
		}

		lines[i] = fn(section, line)

		if m := sectionRE.FindStringSubmatch(lines[i]); m != nil {
			section = m[1]
		}
	}

	return []byte(strings.Join(lines, "\n"))
}

// RewritePathDeps replaces dependency `path = "<oldPath>"` values with
// newPath, in both inline-table deps under [dependencies] (and
// [dev-dependencies], [build-dependencies]) and table-form
// [dependencies.<name>] sections. Returns the rewritten manifest and
// the number of replacements. Zero replacements returns the input
// byte-identical.
func RewritePathDeps(manifest []byte, oldPath, newPath string) ([]byte, int, error) {
	pathRE := regexp.MustCompile(
		`((?:^|[\s{])path\s*=\s*)"` + regexp.QuoteMeta(oldPath) + `"`,
	)
	n := 0

	out := forEachLine(manifest, func(section, line string) string {
		if !isDependencySection(section) {
			return line
		}

		matches := len(pathRE.FindAllString(line, -1))
		if matches == 0 {
			return line
		}

		n += matches

		// ReplaceAllStringFunc (not ReplaceAllString) so a `$` in
		// newPath lands literally instead of being parsed as a
		// submatch reference.
		return pathRE.ReplaceAllStringFunc(line, func(match string) string {
			loc := pathRE.FindStringSubmatchIndex(match)
			prefix := match[loc[2]:loc[3]] // group 1

			return prefix + `"` + newPath + `"`
		})
	})

	if n == 0 {
		return manifest, 0, nil
	}

	return out, n, nil
}

// RenameDepKey renames dependency key oldName to newName: inline-table
// keys (`oldName = { … }` / `oldName = "1"`) in bare dependency
// sections, and table headers ([dependencies.oldName] and dev-/build-
// variants). Keys are matched exactly (start of line, then optional
// spaces, then `=`), never by substring.
func RenameDepKey(manifest []byte, oldName, newName string) ([]byte, int, error) {
	keyRE := regexp.MustCompile(
		`^(\s*)` + regexp.QuoteMeta(oldName) + `(\s*=)`,
	)
	headerRE := regexp.MustCompile(
		`^(\s*\[(?:dependencies|dev-dependencies|build-dependencies)\.)` +
			regexp.QuoteMeta(oldName) + `(\]\s*(?:#.*)?)$`,
	)
	n := 0

	out := forEachLine(manifest, func(section, line string) string {
		if headerRE.MatchString(line) {
			n++

			return headerRE.ReplaceAllString(line, `${1}`+newName+`${2}`)
		}

		// Inline keys only live directly under the bare dependency
		// tables; inside [dependencies.<name>] the keys are
		// path/version/features, not dependency names.
		isBareDepSection := false

		for _, base := range dependencySectionBases {
			if section == base {
				isBareDepSection = true

				break
			}
		}

		if !isBareDepSection || !keyRE.MatchString(line) {
			return line
		}

		n++

		return keyRE.ReplaceAllString(line, `${1}`+newName+`${2}`)
	})

	if n == 0 {
		return manifest, 0, nil
	}

	return out, n, nil
}

// SetPackageName rewrites `name = "…"` inside the [package] section
// only, preserving spacing and trailing comments. Errors if [package]
// has no name key.
func SetPackageName(manifest []byte, newName string) ([]byte, error) {
	nameRE := regexp.MustCompile(`^(\s*name\s*=\s*)"[^"]*"`)
	found := false

	out := forEachLine(manifest, func(section, line string) string {
		if section != "package" || !nameRE.MatchString(line) {
			return line
		}

		found = true

		return nameRE.ReplaceAllString(line, `${1}"`+newName+`"`)
	})

	if !found {
		return nil, fmt.Errorf("no name key found in [package] section")
	}

	return out, nil
}

// ReplaceMember swaps a [workspace] members entry string: the first
// quoted occurrence of oldRel on each [workspace]-section line becomes
// newRel. Returns ok=false (input unchanged) when oldRel is absent.
func ReplaceMember(manifest []byte, oldRel, newRel string) ([]byte, bool, error) {
	needle := `"` + oldRel + `"`
	replacement := `"` + newRel + `"`
	replaced := false

	out := forEachLine(manifest, func(section, line string) string {
		if section != "workspace" || !strings.Contains(line, needle) {
			return line
		}

		replaced = true

		return strings.Replace(line, needle, replacement, 1)
	})

	if !replaced {
		return manifest, false, nil
	}

	return out, true, nil
}

// AddMember appends rel to the [workspace] members array if absent
// (idempotent: a second add returns the input unchanged with
// added=false). Only multiline arrays are supported; a single-line
// `members = ["a"]` array errors with "unsupported single-line members
// array". The inserted entry matches the indentation of existing
// entries (two spaces when the array is empty).
func AddMember(manifest []byte, rel string) ([]byte, bool, error) {
	openRE := regexp.MustCompile(`^\s*members\s*=\s*\[\s*(?:#.*)?$`)
	singleLineRE := regexp.MustCompile(`^\s*members\s*=\s*\[.*\]`)
	closeRE := regexp.MustCompile(`^\s*\]\s*,?\s*(?:#.*)?$`)
	needle := `"` + rel + `"`

	lines := strings.Split(string(manifest), "\n")
	section := ""

	for i, line := range lines {
		if m := sectionRE.FindStringSubmatch(line); m != nil {
			section = m[1]

			continue
		}

		if section != "workspace" {
			continue
		}

		if singleLineRE.MatchString(line) {
			return nil, false, fmt.Errorf("unsupported single-line members array")
		}

		if !openRE.MatchString(line) {
			continue
		}

		indent := "  "

		for j := i + 1; j < len(lines); j++ {
			if closeRE.MatchString(lines[j]) {
				entry := indent + needle + ","
				out := make([]string, 0, len(lines)+1)
				out = append(out, lines[:j]...)
				out = append(out, entry)
				out = append(out, lines[j:]...)

				return []byte(strings.Join(out, "\n")), true, nil
			}

			if strings.Contains(lines[j], needle) {
				return manifest, false, nil
			}

			if strings.TrimSpace(lines[j]) != "" {
				indent = lines[j][:len(lines[j])-len(strings.TrimLeft(lines[j], " \t"))]
			}
		}

		return nil, false, fmt.Errorf("members array opened but never closed")
	}

	return nil, false, fmt.Errorf("no [workspace] members array found")
}
