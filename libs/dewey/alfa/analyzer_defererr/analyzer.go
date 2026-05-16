// Package defererr defines an Analyzer that checks for deferred calls
// whose error return value is silently dropped.
//
// # Analyzer defererr
//
// defererr: check deferred calls do not silently drop errors
//
// A defer statement that calls a function returning an error (or a
// tuple containing an error) discards the error value. This is
// commonly a bug — for example, defer file.Close() silently drops
// write errors. Wrap the call in a closure that checks or assigns
// the error, or suppress with a //defer:err-checked comment.
package analyzer_defererr

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "defererr",
	Doc:      "check deferred calls do not silently drop errors",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeTypes := []ast.Node{(*ast.DeferStmt)(nil)}

	ins.Preorder(nodeTypes, func(n ast.Node) {
		deferStmt := n.(*ast.DeferStmt)

		if hasSuppressComment(pass, deferStmt) {
			return
		}

		call := deferStmt.Call

		// If the deferred expression is a function literal (closure),
		// the error can be handled inside. Skip these — the user has
		// already wrapped the call.
		if _, ok := call.Fun.(*ast.FuncLit); ok {
			return
		}

		if !callReturnsError(pass, call) {
			return
		}

		name := callName(call)
		if isCloseCall(call) {
			pass.ReportRangef(
				deferStmt,
				"deferred call to %s discards its error return value; for read-only fds use files.CloseReadOnly, otherwise wrap with errors.DeferredCloser(&err, x) and switch to a named return",
				name,
			)
		} else {
			pass.ReportRangef(
				deferStmt,
				"deferred call to %s discards its error return value; wrap with one of errors.Deferred* (DeferredCloser, DeferredFlusher, Deferred, ...) and switch to a named return",
				name,
			)
		}
	})

	return nil, nil
}

func callReturnsError(pass *analysis.Pass, call *ast.CallExpr) bool {
	t := pass.TypesInfo.TypeOf(call)
	if t == nil {
		return false
	}

	errorType := types.Universe.Lookup("error").Type()

	// Single return value.
	if types.Identical(t, errorType) {
		return true
	}

	// Tuple return value — check each element.
	tuple, ok := t.(*types.Tuple)
	if !ok {
		return false
	}

	for i := range tuple.Len() {
		if types.Identical(tuple.At(i).Type(), errorType) {
			return true
		}
	}

	return false
}

func isCloseCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "Close"
}

func callName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fn.Sel.Name
	case *ast.Ident:
		return fn.Name
	}

	return "<deferred function>"
}

func hasSuppressComment(pass *analysis.Pass, node ast.Node) bool {
	pos := pass.Fset.Position(node.Pos())

	for _, f := range pass.Files {
		for _, cg := range f.Comments {
			for _, comment := range cg.List {
				cpos := pass.Fset.Position(comment.Pos())
				if cpos.Line == pos.Line && strings.Contains(comment.Text, "//defer:err-checked") {
					return true
				}
			}
		}
	}

	return false
}
