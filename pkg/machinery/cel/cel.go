// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cel provides helpers for working with CEL expressions.
package cel

import (
	"encoding"
	"fmt"
	"math"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/protoenc"
	"go.yaml.in/yaml/v4"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// Expression is a CEL expression that can be marshaled/unmarshaled as part of the resource.
type Expression struct {
	ast        *cel.Ast
	expression *string
}

// Check interfaces.
var (
	_ encoding.TextMarshaler   = Expression{}
	_ encoding.TextUnmarshaler = (*Expression)(nil)
	_ yaml.IsZeroer            = Expression{}
)

// MustExpression panics if the expression cannot be parsed.
func MustExpression(expr Expression, err error) Expression {
	if err != nil {
		panic(err)
	}

	return expr
}

// ParseBooleanExpression parses the expression and asserts the result to boolean.
func ParseBooleanExpression(expression string, env *cel.Env) (Expression, error) {
	ast, err := parseExpressionWithOutputType(expression, env, types.BoolType)
	if err != nil {
		return Expression{}, err
	}

	return Expression{ast: ast}, nil
}

// ParseDoubleExpression parses the expression and asserts the result to float.
func ParseDoubleExpression(expression string, env *cel.Env) (Expression, error) {
	ast, err := parseExpressionWithOutputType(expression, env, types.DoubleType)
	if err != nil {
		return Expression{}, err
	}

	return Expression{ast: ast}, nil
}

// parseExpressionWithOutputType parses the expression and asserts the result to boolean.
func parseExpressionWithOutputType(expression string, env *cel.Env, expectedType *types.Type) (*cel.Ast, error) {
	ast, issues := env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}

	ast, issues = env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}

	if outputType := ast.OutputType(); !outputType.IsExactType(expectedType) {
		return nil, fmt.Errorf("expression output type is %s, expected %s", outputType, expectedType)
	}

	return ast, nil
}

// Merge imlements merge.Mergeable.
func (expr *Expression) Merge(v any) error {
	other, ok := v.(Expression)
	if !ok {
		return fmt.Errorf("unexpected type for expression merge %T", v)
	}

	expr.ast = other.ast
	expr.expression = other.expression

	return nil
}

// ParseBool parses the expression and asserts the result to boolean.
//
// ParseBoolean can be used after unmarshaling the expression from text.
func (expr *Expression) ParseBool(env *cel.Env) error {
	if expr.ast != nil {
		return nil
	}

	if expr.expression == nil {
		panic("expression is not set")
	}

	var err error

	expr.ast, err = parseExpressionWithOutputType(*expr.expression, env, types.BoolType)

	return err
}

// ParseDouble parses the expression and asserts the result to float.
//
// ParseDouble can be used after unmarshaling the expression from text.
func (expr *Expression) ParseDouble(env *cel.Env) error {
	if expr.ast != nil {
		return nil
	}

	if expr.expression == nil {
		panic("expression is not set")
	}

	var err error

	expr.ast, err = parseExpressionWithOutputType(*expr.expression, env, types.DoubleType)

	return err
}

// EvalBool evaluates the expression in the given environment.
func (expr Expression) EvalBool(env *cel.Env, values map[string]any) (bool, error) {
	if err := expr.ParseBool(env); err != nil {
		return false, err
	}

	prog, err := env.Program(expr.ast)
	if err != nil {
		return false, err
	}

	out, _, err := prog.Eval(values)
	if err != nil {
		return false, err
	}

	val, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("expression output type is %s, expected bool", out.Type())
	}

	return val, nil
}

// EvalDouble evaluates the expression in the given environment.
func (expr Expression) EvalDouble(env *cel.Env, values map[string]any) (float64, error) {
	if err := expr.ParseDouble(env); err != nil {
		return math.NaN(), err
	}

	prog, err := env.Program(expr.ast)
	if err != nil {
		return math.NaN(), err
	}

	out, _, err := prog.Eval(values)
	if err != nil {
		return math.NaN(), err
	}

	val, ok := out.Value().(float64)
	if !ok {
		return math.NaN(), fmt.Errorf("expression output type is %s, expected float64", out.Type())
	}

	return val, nil
}

// MarshalText marshals the expression to text.
func (expr Expression) MarshalText() ([]byte, error) {
	if expr.expression != nil {
		return []byte(*expr.expression), nil
	}

	if expr.ast != nil {
		repr, err := cel.AstToString(expr.ast)
		if err != nil {
			return nil, err
		}

		return []byte(repr), nil
	}

	return nil, nil
}

// UnmarshalText unmarshals the expression from text.
func (expr *Expression) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	expr.expression = pointer.To(string(data))

	return nil
}

// String implements fmt.Stringer.
func (expr Expression) String() string {
	b, err := expr.MarshalText()
	if err != nil {
		return "ERROR: " + err.Error()
	}

	return string(b)
}

// IsZero returns true if the expression is zero.
func (expr Expression) IsZero() bool {
	return expr.ast == nil && expr.expression == nil
}

// MarshalProto marshals the expression to proto.
func (expr Expression) MarshalProto() ([]byte, error) {
	if expr.ast == nil {
		return nil, nil
	}

	pbExpr, err := cel.AstToCheckedExpr(expr.ast)
	if err != nil {
		return nil, err
	}

	return proto.Marshal(pbExpr)
}

// UnmarshalProto unmarshals the expression from proto.
func (expr *Expression) UnmarshalProto(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	pbExpr := &exprpb.CheckedExpr{}
	if err := proto.Unmarshal(data, pbExpr); err != nil {
		return err
	}

	expr.ast = cel.CheckedExprToAst(pbExpr)

	return nil
}

func init() {
	protoenc.RegisterEncoderDecoder(
		func(v Expression) ([]byte, error) {
			return v.MarshalProto()
		},
		func(slc []byte) (Expression, error) {
			var v Expression

			err := v.UnmarshalProto(slc)

			return v, err
		},
	)
}
