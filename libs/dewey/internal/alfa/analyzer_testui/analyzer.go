// Package testui defines an Analyzer that flags stdlib *testing.T
// usage where dewey's test_ui.T is intended.
//
// # Analyzer testui
//
// testui: prefer dewey's test_ui.T over stdlib *testing.T
//
// The project standardizes on dewey's test_ui.T — a wrapper that
// embeds *testing.T and adds richer assertion and printing helpers —
// for test code. Helper functions, fixture structs, and test-support
// interfaces should take test_ui.T rather than the bare stdlib type.
// This analyzer reports function parameters, function results,
// interface method signatures, and struct fields typed as stdlib
// *testing.T.
//
// The canonical Go test entry points are exempt: the testing runtime
// mandates the exact signature func TestXxx(t *testing.T), and
// test_ui.T is constructed from that runtime-provided *testing.T
// (e.g. &test_ui.T{T: t}). The analyzer therefore does not flag the
// first parameter of a free function whose name is a test entry point
// (Test, TestXxx — i.e. the "Test" prefix not followed by a lowercase
// letter, matching go test's own discovery rule). Methods and
// non-entry-point helpers are not exempt.
//
// Other seams that genuinely require stdlib *testing.T — third-party
// helpers such as testify's require/assert functions, or subtest
// closures handed a *testing.T by the stdlib runtime — can be
// suppressed with a //testui:allow comment on the same line or on the
// line immediately above.
package analyzer_testui

import (
	"go/ast"
	"go/types"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const message = "stdlib *testing.T used here; prefer dewey's test_ui.T (it embeds *testing.T and adds assertion/printing helpers), or suppress with //testui:allow at interop boundaries"

var Analyzer = &analysis.Analyzer{
	Name:     "testui",
	Doc:      "prefer dewey's test_ui.T over stdlib *testing.T",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeTypes := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.InterfaceType)(nil),
		(*ast.StructType)(nil),
	}

	ins.Preorder(nodeTypes, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.FuncDecl:
			// The first parameter of a canonical test entry point is the
			// runtime-provided *testing.T and cannot change type; every
			// other parameter, and all results, are fair game.
			reportTestingTFields(pass, node.Type.Params, isTestEntrypoint(node))
			reportTestingTFields(pass, node.Type.Results, false)
		case *ast.InterfaceType:
			for _, method := range node.Methods.List {
				funcType, ok := method.Type.(*ast.FuncType)
				if !ok {
					continue
				}
				reportTestingTFields(pass, funcType.Params, false)
				reportTestingTFields(pass, funcType.Results, false)
			}
		case *ast.StructType:
			reportTestingTFields(pass, node.Fields, false)
		}
	})

	return nil, nil
}

// reportTestingTFields reports each field in a field list whose type is
// stdlib *testing.T and is not suppressed. When skipFirst is set, the
// first field is left unreported — used to exempt the runtime-provided
// receiver of a test entry point.
func reportTestingTFields(pass *analysis.Pass, fields *ast.FieldList, skipFirst bool) {
	if fields == nil {
		return
	}

	for i, field := range fields.List {
		if skipFirst && i == 0 {
			continue
		}

		if !isStdlibTestingT(pass.TypesInfo.TypeOf(field.Type)) {
			continue
		}

		if hasAllowComment(pass, field) {
			continue
		}

		pass.ReportRangef(field.Type, message)
	}
}

// isStdlibTestingT reports whether t is stdlib testing.T or *testing.T.
// Test code overwhelmingly uses the pointer form, but an embedded value
// field (struct{ testing.T }) is matched too.
func isStdlibTestingT(t types.Type) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}

	return obj.Pkg().Path() == "testing" && obj.Name() == "T"
}

// isTestEntrypoint reports whether decl is a Go test entry point whose
// first parameter is the runtime-provided *testing.T: a free function
// (no receiver) whose name go test would discover as a test. Methods
// are never entry points — go test only runs package-level functions.
func isTestEntrypoint(decl *ast.FuncDecl) bool {
	if decl.Recv != nil || decl.Name == nil {
		return false
	}

	return isTestFuncName(decl.Name.Name)
}

// isTestFuncName mirrors go test's discovery rule for TestXxx: the name
// has the "Test" prefix and the following rune (if any) is not a
// lowercase letter. "Testify" is therefore not an entry point — go test
// would not run it either, so a helper by that name should be flagged.
func isTestFuncName(name string) bool {
	const prefix = "Test"

	if !strings.HasPrefix(name, prefix) {
		return false
	}

	rest := name[len(prefix):]
	if rest == "" {
		return true
	}

	r, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLower(r)
}

// hasAllowComment honors the //testui:allow directive at the end of the
// flagged line or on the line immediately above it.
func hasAllowComment(pass *analysis.Pass, node ast.Node) bool {
	pos := pass.Fset.Position(node.Pos())

	for _, f := range pass.Files {
		for _, cg := range f.Comments {
			for _, comment := range cg.List {
				cpos := pass.Fset.Position(comment.Pos())
				if (cpos.Line == pos.Line || cpos.Line == pos.Line-1) && strings.Contains(comment.Text, "//testui:allow") {
					return true
				}
			}
		}
	}

	return false
}
