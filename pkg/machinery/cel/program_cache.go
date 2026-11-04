// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cel

import (
	"runtime"
	"sync"
	"weak"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
)

// programKey identifies a compiled program in the cache.
//
// Expressions are values, but their copies share the parsed AST pointer (or the source text pointer,
// if the expression was unmarshaled from text and not parsed yet), so the program is cached by that pointer.
// The pointers are weak, so that the cache doesn't keep the expressions alive: once the expression
// is garbage collected, its program is removed from the cache.
type programKey struct {
	env *cel.Env

	// exactly one of ast or text is set
	ast  weak.Pointer[cel.Ast]
	text weak.Pointer[string]

	// expectedType is the expected output type, used only for text expressions (as they are type-checked on parsing).
	expectedType *types.Type
}

// programCache holds compiled programs (cel.Program), which are safe for concurrent use.
var programCache sync.Map // map[programKey]cel.Program

// program returns the compiled program for the expression, compiling and caching it on the first call.
func (expr Expression) program(env *cel.Env, expectedType *types.Type) (cel.Program, error) { //nolint:ireturn
	key := programKey{env: env}

	switch {
	case expr.ast != nil:
		key.ast = weak.Make(expr.ast)
	case expr.expression != nil:
		key.text = weak.Make(expr.expression)
		key.expectedType = expectedType
	default:
		panic("expression is not set")
	}

	if prog, ok := programCache.Load(key); ok {
		return prog.(cel.Program), nil //nolint:forcetypeassert
	}

	ast := expr.ast

	if ast == nil {
		var err error

		// errors are not cached, so an invalid expression is re-parsed on each evaluation (and fails again)
		ast, err = parseExpressionWithOutputType(*expr.expression, env, expectedType)
		if err != nil {
			return nil, err
		}
	}

	prog, err := env.Program(ast)
	if err != nil {
		return nil, err
	}

	if actual, loaded := programCache.LoadOrStore(key, prog); loaded {
		// lost the race to another goroutine, use the program it compiled
		return actual.(cel.Program), nil //nolint:forcetypeassert
	}

	// the program references the internal representation of the AST, but neither the *cel.Ast wrapper
	// nor the source text pointer, so the key object can be collected while the program is cached
	if expr.ast != nil {
		runtime.AddCleanup(expr.ast, deleteProgram, key)
	} else {
		runtime.AddCleanup(expr.expression, deleteProgram, key)
	}

	return prog, nil
}

func deleteProgram(key programKey) {
	programCache.Delete(key)
}
