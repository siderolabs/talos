// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cel

import (
	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"

	"github.com/siderolabs/talos/pkg/machinery/cel/internal/weakcache"
)

// textProgramKey identifies a program compiled from the expression text.
//
// The expected type is part of the key, as text expressions are type-checked on parsing.
type textProgramKey struct {
	env          *cel.Env
	expectedType *types.Type
}

// Compiled programs (cel.Program), which are safe for concurrent use.
//
// Expressions are values, but their copies share the parsed AST pointer (or the source text pointer,
// if the expression was unmarshaled from text and not parsed yet), so the program is cached by that pointer.
// The cache doesn't keep the expressions alive: once the expression is garbage collected, its programs are removed.
//
// The program references the internal representation of the AST, but neither the *cel.Ast wrapper
// nor the source text, so the cached program doesn't keep the expression alive.
var (
	astPrograms  weakcache.Cache[cel.Ast, *cel.Env, cel.Program]
	textPrograms weakcache.Cache[string, textProgramKey, cel.Program]
)

// program returns the compiled program for the expression, compiling and caching it on the first call.
func (expr Expression) program(env *cel.Env, expectedType *types.Type) (cel.Program, error) { //nolint:ireturn
	switch {
	case expr.ast != nil:
		if prog, ok := astPrograms.Get(expr.ast, env); ok {
			return prog, nil
		}

		prog, err := env.Program(expr.ast)
		if err != nil {
			return nil, err
		}

		return astPrograms.Store(expr.ast, env, prog), nil
	case expr.expression != nil:
		key := textProgramKey{env: env, expectedType: expectedType}

		if prog, ok := textPrograms.Get(expr.expression, key); ok {
			return prog, nil
		}

		// errors are not cached, so an invalid expression is re-parsed on each evaluation (and fails again)
		ast, err := parseExpressionWithOutputType(*expr.expression, env, expectedType)
		if err != nil {
			return nil, err
		}

		prog, err := env.Program(ast)
		if err != nil {
			return nil, err
		}

		return textPrograms.Store(expr.expression, key, prog), nil
	default:
		panic("expression is not set")
	}
}
