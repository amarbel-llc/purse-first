package dagnabit

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/packages"
)

// buildDocMap collects doc comments from a parsed Go package, keyed by
// the exported symbol name. Handles top-level [ast.FuncDecl] and the
// var/const/type specs inside [ast.GenDecl]. For grouped GenDecls,
// per-spec docs win over the group-level doc; the group-level doc
// falls back when there's only one spec (single-line declarations that
// the syntax happens to wrap in a 1-element group).
//
// Methods, unexported names, blank identifiers, and symbols without
// any leading doc comment are skipped — the returned map only contains
// names that actually have something to propagate.
func buildDocMap(pkg *packages.Package) map[string]*ast.CommentGroup {
	docs := make(map[string]*ast.CommentGroup)
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil || d.Name == nil || !d.Name.IsExported() {
					continue
				}
				if d.Doc != nil {
					docs[d.Name.Name] = d.Doc
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if !s.Name.IsExported() {
							continue
						}
						doc := s.Doc
						if doc == nil && len(d.Specs) == 1 {
							doc = d.Doc
						}
						if doc != nil {
							docs[s.Name.Name] = doc
						}
					case *ast.ValueSpec:
						for _, name := range s.Names {
							if !name.IsExported() {
								continue
							}
							doc := s.Doc
							if doc == nil && len(d.Specs) == 1 {
								doc = d.Doc
							}
							if doc != nil {
								docs[name.Name] = doc
							}
						}
					}
				}
			}
		}
	}
	return docs
}

// docCommentLines returns the textual lines of a doc comment group with
// the leading `//` or `/* … */` markers stripped, ready to be fed to
// jennifer's `f.Comment(line)` (which re-adds the `// ` prefix).
//
// Compiler directives (`//go:…`, `//line …`, `//export …`) are
// filtered out: they apply to the original symbol's implementation,
// not to a re-export alias. Propagating them to the facade would be
// either harmless (best case) or actively wrong (worst case, e.g.
// `//go:noinline` on a `var` is a syntax error).
func docCommentLines(cg *ast.CommentGroup) []string {
	if cg == nil {
		return nil
	}
	var out []string
	for _, c := range cg.List {
		text := c.Text
		switch {
		case strings.HasPrefix(text, "//"):
			inner := strings.TrimPrefix(text, "//")
			inner = strings.TrimPrefix(inner, " ")
			if isCompilerDirective(inner) {
				continue
			}
			out = append(out, inner)
		case strings.HasPrefix(text, "/*"):
			inner := strings.TrimPrefix(text, "/*")
			inner = strings.TrimSuffix(inner, "*/")
			for _, line := range strings.Split(inner, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || isCompilerDirective(line) {
					continue
				}
				out = append(out, line)
			}
		}
	}
	return out
}

// isCompilerDirective recognises the `//go:…`, `//line …`, and
// `//export …` directives. The argument is the comment body with the
// leading `//` (and at most one space) already stripped.
func isCompilerDirective(body string) bool {
	return strings.HasPrefix(body, "go:") ||
		strings.HasPrefix(body, "line ") ||
		strings.HasPrefix(body, "export ")
}
