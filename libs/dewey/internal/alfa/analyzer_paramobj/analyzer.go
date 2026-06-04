// Package paramobj defines an Analyzer that flags clusters of
// functions sharing the same parameter list as candidates for a
// parameter-object struct.
//
// # Analyzer paramobj
//
// paramobj: suggest a parameter struct when many functions share a signature
//
// When N functions or methods all take the same parameter type list,
// threading an extra field means editing every one and the signatures
// drift. A struct collapses the churn and documents the cohesive
// parameter set. This analyzer groups declarations by their parameter
// type list (order-sensitive, receiver ignored) and reports each group
// whose size reaches the -min-group threshold.
//
// The check is advisory — extracting a parameter object is a judgment
// call, so there is no autofix. Noise is controlled three ways: only
// declarations with at least -min-params parameters are considered
// (which excludes idiomatic two-parameter shapes such as
// (w http.ResponseWriter, r *http.Request) and
// (ctx context.Context, args json.RawMessage)); only groups reaching
// -min-group are reported; and any single declaration opts out with a
// //paramobj:allow comment on its signature line.
package analyzer_paramobj

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	defaultMinGroup  = 3
	defaultMinParams = 3
)

var (
	minGroup  = defaultMinGroup
	minParams = defaultMinParams
)

var Analyzer = &analysis.Analyzer{
	Name:     "paramobj",
	Doc:      "suggest a parameter struct when many functions share a signature",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func init() {
	Analyzer.Flags.IntVar(&minGroup, "min-group", defaultMinGroup,
		"minimum number of declarations sharing a parameter list before reporting")
	Analyzer.Flags.IntVar(&minParams, "min-params", defaultMinParams,
		"minimum parameter count for a declaration to be considered")
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	groups := make(map[string][]*ast.FuncDecl)

	ins.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		decl := n.(*ast.FuncDecl)

		fn, ok := pass.TypesInfo.Defs[decl.Name].(*types.Func)
		if !ok || fn == nil {
			return
		}

		sig, ok := fn.Type().(*types.Signature)
		if !ok {
			return
		}

		if sig.Params().Len() < minParams {
			return
		}

		if hasAllowComment(pass, decl) {
			return
		}

		key := signatureKey(sig, pass.Pkg)
		groups[key] = append(groups[key], decl)
	})

	for _, decls := range groups {
		if len(decls) < minGroup {
			continue
		}

		for _, decl := range decls {
			pass.ReportRangef(
				decl.Name,
				"function %q has a parameter list shared by %d declarations in this package; consider extracting a parameter struct (or suppress with //paramobj:allow)",
				decl.Name.Name,
				len(decls),
			)
		}
	}

	return nil, nil
}

// signatureKey renders a signature's parameter type list as an
// order-sensitive key. The receiver, parameter names, and result types
// are all excluded — only the ordered parameter types identify a group.
func signatureKey(sig *types.Signature, pkg *types.Package) string {
	params := sig.Params()
	parts := make([]string, params.Len())
	qualifier := types.RelativeTo(pkg)

	for i := range params.Len() {
		parts[i] = types.TypeString(params.At(i).Type(), qualifier)
	}

	return strings.Join(parts, ", ")
}

func hasAllowComment(pass *analysis.Pass, node ast.Node) bool {
	pos := pass.Fset.Position(node.Pos())

	for _, f := range pass.Files {
		for _, cg := range f.Comments {
			for _, comment := range cg.List {
				cpos := pass.Fset.Position(comment.Pos())
				if cpos.Line == pos.Line && strings.Contains(comment.Text, "//paramobj:allow") {
					return true
				}
			}
		}
	}

	return false
}
