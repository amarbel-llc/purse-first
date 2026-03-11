# macOS Path Resolution

## Problem

On macOS, `/var` is a symlink to `/private/var` and `/tmp` is a symlink to
`/private/tmp`. Go's `filepath.EvalSymlinks` requires the target path to exist
-- when called on a non-existent path, it returns an error and the caller
typically falls back to the raw (unresolved) path.

This creates false mismatches when comparing paths: one path resolves through
the symlink (e.g., `/private/var/folders/...`) while the other retains the
unresolved form (e.g., `/var/folders/...`). The two paths refer to the same
location but string comparison says they differ.

## When It Matters

- **Path containment checks** (is path A inside directory B?)
- **Prefix matching** (does path start with a known root?)
- **Any comparison** where one path may not exist yet

## When It Doesn't Matter

- Resolving `os.Executable()` -- the binary always exists
- Resolving paths you've confirmed exist via `os.Stat`

## Pattern: Walk-Up-Ancestors Resolution

Walk up the directory tree until finding an existing ancestor, resolve symlinks
there, then re-append the non-existent suffix:

```go
// From packages/spinclass/internal/hooks/hooks.go

func resolvePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved
	}

	// Path doesn't fully exist -- walk up until we find an existing ancestor,
	// resolve symlinks there, then re-append the non-existent suffix.
	cleaned := filepath.Clean(path)
	var suffix []string
	current := cleaned
	for {
		parent := filepath.Dir(current)
		suffix = append(suffix, filepath.Base(current))
		if parent == current {
			break
		}
		if resolved, err := filepath.EvalSymlinks(parent); err == nil {
			// Reverse suffix and join.
			for i, j := 0, len(suffix)-1; i < j; i, j = i+1, j-1 {
				suffix[i], suffix[j] = suffix[j], suffix[i]
			}
			return filepath.Join(append([]string{resolved}, suffix...)...)
		}
		current = parent
	}

	return cleaned
}
```

## Key Points

- Always resolve **both** sides of a path comparison, not just one
- The walk-up pattern handles arbitrary nesting depth (not just `/var` or `/tmp`)
- On Linux this is a no-op -- `filepath.EvalSymlinks` succeeds on the first call
  since `/var` and `/tmp` are real directories
