package dagnabit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// rewriteConsumers walks up from exporter.Dir to find the workspace root
// (the directory containing go.work, or exporter.Dir if no go.work is
// found), then walks every .go file outside the dewey module looking for
// imports of internalImportPath and rewrites them to facadeImportPath.
//
// Files inside the dewey module itself (i.e., under exporter.Dir) are
// skipped — internal callers have first-class access to the internal path
// and shouldn't be forced through the facade. Only external workspace
// consumers are touched.
//
// Returns nil on success even when no files needed rewriting.
func (exporter *Exporter) rewriteConsumers(
	internalImportPath, facadeImportPath string,
) error {
	searchRoot := findWorkspaceRoot(exporter.Dir)

	moduleAbs, err := filepath.Abs(exporter.Dir)
	if err != nil {
		return fmt.Errorf("resolving module dir: %w", err)
	}

	return filepath.Walk(searchRoot, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}

		if info.IsDir() {
			base := filepath.Base(path)

			switch base {
			case ".git", "vendor", "node_modules", "build", "result", ".tmp", ".direnv", "testdata":
				return filepath.SkipDir
			}

			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}

			if abs == moduleAbs {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Cheap pre-check: skip files that don't textually mention the
		// internal path. Avoids parsing every .go file in the workspace.
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if !strings.Contains(string(content), internalImportPath) {
			return nil
		}

		if err := rewriteFileImports(path, internalImportPath, facadeImportPath); err != nil {
			return fmt.Errorf("rewriting %s: %w", path, err)
		}

		fmt.Printf("rewrote consumer: %s\n", path)

		return nil
	})
}

// findWorkspaceRoot walks up from dir looking for a go.work file. Returns
// the directory containing go.work, or dir itself if no go.work is found
// within 16 levels (a defensive cap).
func findWorkspaceRoot(dir string) string {
	cur, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}

	for i := 0; i < 16; i++ {
		if _, err := os.Stat(filepath.Join(cur, "go.work")); err == nil {
			return cur
		}

		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}

		cur = parent
	}

	return dir
}
