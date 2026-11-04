// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cel

import (
	"weak"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
)

// IsProgramCached returns true if the compiled program of the expression is in the cache.
func IsProgramCached(expr Expression, env *cel.Env, expectedType *types.Type) bool {
	key := programKey{env: env}

	if expr.ast != nil {
		key.ast = weak.Make(expr.ast)
	} else {
		key.text = weak.Make(expr.expression)
		key.expectedType = expectedType
	}

	_, ok := programCache.Load(key)

	return ok
}

// ProgramCacheLen returns the number of cached programs.
func ProgramCacheLen() int {
	n := 0

	programCache.Range(func(any, any) bool {
		n++

		return true
	})

	return n
}
