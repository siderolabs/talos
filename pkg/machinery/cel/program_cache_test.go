// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cel_test

import (
	"runtime"
	"sync"
	"testing"

	celgo "cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/cel"
)

func testEnv(t testing.TB) *celgo.Env {
	t.Helper()

	env, err := celgo.NewEnv(celgo.Variable("x", types.IntType))
	require.NoError(t, err)

	return env
}

func unmarshalExpression(t testing.TB, text string) cel.Expression {
	t.Helper()

	var expr cel.Expression

	require.NoError(t, expr.UnmarshalText([]byte(text)))

	return expr
}

// evalAllocs returns the number of allocations of a single call.
//
// The tests using it must not run in parallel with other tests.
func evalAllocs(t *testing.T, eval func() error) uint64 {
	t.Helper()

	var before, after runtime.MemStats

	runtime.ReadMemStats(&before)

	err := eval()

	runtime.ReadMemStats(&after)

	require.NoError(t, err)

	return after.Mallocs - before.Mallocs
}

// evalBool returns a function which evaluates the expression and checks the result.
func evalBool(t *testing.T, expr cel.Expression, env *celgo.Env, x int, expected bool) func() error {
	t.Helper()

	return func() error {
		result, err := expr.EvalBool(env, map[string]any{"x": x})
		if err != nil {
			return err
		}

		assert.Equal(t, expected, result)

		return nil
	}
}

// Evaluation which compiles the program allocates way more (> 20 allocations) than the one using the cached program (2 allocations).
const compileAllocsFactor = 4

//nolint:paralleltest // measures allocations
func TestProgramCacheParsed(t *testing.T) {
	env := testEnv(t)

	expr := cel.MustExpression(cel.ParseBooleanExpression("x > 10", env))

	compiled := evalAllocs(t, evalBool(t, expr, env, 5, false))
	cached := evalAllocs(t, evalBool(t, expr, env, 15, true))

	// the program is cached
	assert.Greater(t, compiled, compileAllocsFactor*cached)

	// copies of the expression (e.g. from resource DeepCopy) share the cached program
	exprCopy := expr

	assert.Less(t, evalAllocs(t, evalBool(t, exprCopy, env, 15, true)), compileAllocsFactor*cached)

	// the same expression in a different environment is compiled separately, and cached
	otherEnv := testEnv(t)

	assert.Greater(t, evalAllocs(t, evalBool(t, expr, otherEnv, 15, true)), compileAllocsFactor*cached)
	assert.Less(t, evalAllocs(t, evalBool(t, expr, otherEnv, 15, true)), compileAllocsFactor*cached)
}

//nolint:paralleltest // measures allocations
func TestProgramCacheText(t *testing.T) {
	env := testEnv(t)

	// expression unmarshaled from the machine config is not parsed until evaluated
	expr := unmarshalExpression(t, "double(x) * 1.5")

	evalDouble := func() error {
		result, err := expr.EvalDouble(env, map[string]any{"x": 2})
		if err != nil {
			return err
		}

		assert.InDelta(t, 3.0, result, 1e-9)

		return nil
	}

	compiled := evalAllocs(t, evalDouble)
	cached := evalAllocs(t, evalDouble)

	// the program is cached
	assert.Greater(t, compiled, compileAllocsFactor*cached)

	// the cache key includes the expected type, so the type check still happens (on every evaluation, as errors are not cached)
	for range 2 {
		allocs := evalAllocs(t, func() error {
			_, err := expr.EvalBool(env, map[string]any{"x": 2})
			assert.ErrorContains(t, err, "expression output type is double, expected bool")

			return nil
		})

		assert.Greater(t, allocs, compileAllocsFactor*cached)
	}
}

func TestProgramCacheErrors(t *testing.T) {
	t.Parallel()

	env := testEnv(t)

	expr := unmarshalExpression(t, "x >")

	for range 2 {
		_, err := expr.EvalBool(env, map[string]any{"x": 2})
		require.Error(t, err)
	}

	assert.Panics(t, func() {
		cel.Expression{}.EvalBool(env, nil) //nolint:errcheck
	})
}

func TestProgramCacheConcurrent(t *testing.T) {
	t.Parallel()

	env := testEnv(t)

	expr := unmarshalExpression(t, "x % 2 == 0")

	var wg sync.WaitGroup

	for i := range 16 {
		wg.Go(func() {
			for j := range 100 {
				x := i*100 + j

				result, err := expr.EvalBool(env, map[string]any{"x": x})
				assert.NoError(t, err)
				assert.Equal(t, x%2 == 0, result)
			}
		})
	}

	wg.Wait()
}

func TestProgramCacheAllocs(t *testing.T) {
	env := testEnv(t)
	values := map[string]any{"x": 15}

	expr := cel.MustExpression(cel.ParseBooleanExpression("x > 10 && x < 20", env))
	text := unmarshalExpression(t, "x > 10 && x < 20")

	// warm up the cache
	for _, e := range []cel.Expression{expr, text} {
		_, err := e.EvalBool(env, values)
		require.NoError(t, err)
	}

	uncached := testing.AllocsPerRun(100, func() {
		ast, issues := env.Compile("x > 10 && x < 20")
		require.NoError(t, issues.Err())

		prog, err := env.Program(ast)
		require.NoError(t, err)

		_, _, err = prog.Eval(values)
		require.NoError(t, err)
	})

	for name, e := range map[string]cel.Expression{"parsed": expr, "text": text} {
		cached := testing.AllocsPerRun(100, func() {
			_, err := e.EvalBool(env, values)
			require.NoError(t, err)
		})

		t.Logf("%s: cached %.0f allocs, uncached %.0f allocs", name, cached, uncached)

		// evaluation itself allocates a bit, but way less than compiling the program
		assert.Less(t, cached*10, uncached, name)
	}
}

func BenchmarkEvalBool(b *testing.B) {
	env := testEnv(b)
	values := map[string]any{"x": 15}

	for name, expr := range map[string]cel.Expression{
		"parsed": cel.MustExpression(cel.ParseBooleanExpression("x > 10 && x < 20", env)),
		"text":   unmarshalExpression(b, "x > 10 && x < 20"),
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				if _, err := expr.EvalBool(env, values); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
