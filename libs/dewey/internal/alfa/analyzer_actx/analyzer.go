// Package actx defines an Analyzer that flags stdlib context.Context
// usage where dewey's ActiveContext is intended.
//
// # Analyzer actx
//
// actx: prefer dewey's ActiveContext over stdlib context.Context
//
// The project standardizes on dewey's ActiveContext
// (errors.ActiveContext / interfaces.ActiveContext) for its
// lifecycle and error-cancellation semantics; stdlib context.Context
// should be the exception, not the default. This analyzer reports
// function parameters, function results, and struct fields typed as
// stdlib context.Context.
//
// Some seams must use stdlib context.Context because they integrate
// with third-party or framework APIs that require it — go-mcp tool
// handlers, exec.CommandContext, net/http, gRPC, and so on. Suppress
// those interop boundaries with a //actx:allow comment on the same
// line.
package analyzer_actx

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "actx",
	Doc:      "prefer dewey's ActiveContext over stdlib context.Context",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeTypes := []ast.Node{
		(*ast.FuncType)(nil),
		(*ast.StructType)(nil),
	}

	ins.Preorder(nodeTypes, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.FuncType:
			reportContextFields(pass, node.Params)
			reportContextFields(pass, node.Results)
		case *ast.StructType:
			reportContextFields(pass, node.Fields)
		}
	})

	return nil, nil
}

// reportContextFields reports each field in a field list whose type is
// exactly stdlib context.Context and is not suppressed.
func reportContextFields(pass *analysis.Pass, fields *ast.FieldList) {
	if fields == nil {
		return
	}

	for _, field := range fields.List {
		if !isStdlibContext(pass.TypesInfo.TypeOf(field.Type)) {
			continue
		}

		if hasAllowComment(pass, field) {
			continue
		}

		pass.ReportRangef(
			field.Type,
			"stdlib context.Context used here; prefer dewey's errors.ActiveContext (interfaces.ActiveContext) for lifecycle/error-cancellation semantics, or suppress with //actx:allow at interop boundaries",
		)
	}
}

func isStdlibContext(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}

	return obj.Pkg().Path() == "context" && obj.Name() == "Context"
}

func hasAllowComment(pass *analysis.Pass, node ast.Node) bool {
	pos := pass.Fset.Position(node.Pos())

	for _, f := range pass.Files {
		for _, cg := range f.Comments {
			for _, comment := range cg.List {
				cpos := pass.Fset.Position(comment.Pos())
				if cpos.Line == pos.Line && strings.Contains(comment.Text, "//actx:allow") {
					return true
				}
			}
		}
	}

	return false
}
