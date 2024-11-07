// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
)

// Builder allows building CEL expressions programmatically.
type Builder struct {
	ast.ExprFactory
	env    *cel.Env
	nextID int64
}

// NewBuilder creates a new builder.
func NewBuilder(env *cel.Env) *Builder {
	return &Builder{
		ExprFactory: ast.NewExprFactory(),
		env:         env,
	}
}

// NextID returns the next unique ID.
func (b *Builder) NextID() int64 {
	b.nextID++

	return b.nextID
}

// ToBooleanExpression converts the AST to a boolean expression.
func (b *Builder) ToBooleanExpression(expr ast.Expr) (*Expression, error) {
	rawAst := ast.NewAST(expr, nil)

	pbAst, err := ast.ToProto(rawAst)
	if err != nil {
		return nil, err
	}

	celAst, err := cel.CheckedExprToAstWithSource(pbAst, common.NewTextSource(""))
	if err != nil {
		return nil, err
	}

	var issues *cel.Issues

	celAst, issues = b.env.Check(celAst)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}

	if outputType := celAst.OutputType(); !outputType.IsExactType(types.BoolType) {
		return nil, fmt.Errorf("expression output type is %s, expected bool", outputType)
	}

	return &Expression{
		ast: celAst,
	}, nil
}
