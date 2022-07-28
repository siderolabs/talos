// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// parseAst parses the given Go source file and returns the AST.
func parseAst(fset *token.FileSet, path string) (*ast.File, error) {
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// findAllStructs with comments finds all structs in the given AST.
func findAllStructs(f ast.Node) []*ast.TypeSpec {
	var nodes []*ast.TypeSpec

	ast.Inspect(f, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			return true
		}

		typeSpecs := filter(genDecl.Specs, typeSpecFilter)

		switch len(typeSpecs) {
		case 0:
			return false
		case 1:
			if isTarget(genDecl.Doc) || isTarget(typeSpecs[0].Doc) {
				nodes = append(nodes, typeSpecs[0])
			}

			return false
		default:
			for _, typeSpec := range typeSpecs {
				if isTarget(typeSpec.Doc) {
					nodes = append(nodes, typeSpec)
				}
			}

			return false
		}
	})

	return nodes
}

func typeSpecFilter(n ast.Spec) (*ast.TypeSpec, bool) {
	typeSpec, ok := n.(*ast.TypeSpec)
	if !ok {
		return typeSpec, false
	}

	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return typeSpec, false
	}

	if structType.Fields.NumFields() == 0 {
		return typeSpec, false
	}

	if !isCapitalCase(typeSpec.Name.Name) {
		return typeSpec, false
	}

	return typeSpec, true
}

func isTarget(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}

	for _, c := range doc.List {
		if c.Text == "//gotagsrewrite:gen" {
			return true
		}
	}

	return false
}

func filter[T, V any](slc []T, f func(n T) (V, bool)) []V {
	var result []V

	for _, v := range slc {
		res, ok := f(v)
		if ok {
			result = append(result, res)
		}
	}

	return result
}
