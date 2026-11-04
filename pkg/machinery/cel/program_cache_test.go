// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cel_test

import (
	"runtime"
	"sync"
	"testing"
	"time"

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

func TestProgramCacheParsed(t *testing.T) {
	t.Parallel()

	env := testEnv(t)

	expr := cel.MustExpression(cel.ParseBooleanExpression("x > 10", env))

	assert.False(t, cel.IsProgramCached(expr, env, types.BoolType))

	for _, tc := range []struct {
		x        int
		expected bool
	}{
		{x: 5, expected: false},
		{x: 15, expected: true},
	} {
		result, err := expr.EvalBool(env, map[string]any{"x": tc.x})
		require.NoError(t, err)
		assert.Equal(t, tc.expected, result)
	}

	assert.True(t, cel.IsProgramCached(expr, env, types.BoolType))

	// copies of the expression (e.g. from resource DeepCopy) share the cached program
	exprCopy := expr

	assert.True(t, cel.IsProgramCached(exprCopy, env, types.BoolType))

	// the same expression in a different environment is compiled separately
	otherEnv := testEnv(t)

	assert.False(t, cel.IsProgramCached(expr, otherEnv, types.BoolType))

	result, err := expr.EvalBool(otherEnv, map[string]any{"x": 15})
	require.NoError(t, err)
	assert.True(t, result)

	assert.True(t, cel.IsProgramCached(expr, otherEnv, types.BoolType))
}

func TestProgramCacheText(t *testing.T) {
	t.Parallel()

	env := testEnv(t)

	// expression unmarshaled from the machine config is not parsed until evaluated
	expr := unmarshalExpression(t, "double(x) * 1.5")

	result, err := expr.EvalDouble(env, map[string]any{"x": 2})
	require.NoError(t, err)
	assert.InDelta(t, 3.0, result, 1e-9)

	assert.True(t, cel.IsProgramCached(expr, env, types.DoubleType))

	// the cache key includes the expected type, so the type check still happens
	assert.False(t, cel.IsProgramCached(expr, env, types.BoolType))

	_, err = expr.EvalBool(env, map[string]any{"x": 2})
	require.ErrorContains(t, err, "expression output type is double, expected bool")

	assert.False(t, cel.IsProgramCached(expr, env, types.BoolType))
}

func TestProgramCacheErrors(t *testing.T) {
	t.Parallel()

	env := testEnv(t)

	expr := unmarshalExpression(t, "x >")

	for range 2 {
		_, err := expr.EvalBool(env, map[string]any{"x": 2})
		require.Error(t, err)
	}

	assert.False(t, cel.IsProgramCached(expr, env, types.BoolType))

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

	assert.True(t, cel.IsProgramCached(expr, env, types.BoolType))
}

//nolint:paralleltest // relies on the global cache size, must not run in parallel with other cache tests
func TestProgramCacheCleanup(t *testing.T) {
	env := testEnv(t)

	before := cel.ProgramCacheLen()

	func() {
		for i := range 10 {
			parsed := cel.MustExpression(cel.ParseBooleanExpression("x > "+string(rune('0'+i)), env))
			text := unmarshalExpression(t, "x < "+string(rune('0'+i)))

			for _, expr := range []cel.Expression{parsed, text} {
				_, err := expr.EvalBool(env, map[string]any{"x": 5})
				require.NoError(t, err)
			}
		}
	}()

	assert.GreaterOrEqual(t, cel.ProgramCacheLen(), before+20)

	// once the expressions are garbage collected, their programs are dropped from the cache
	assert.Eventually(t, func() bool {
		runtime.GC()

		return cel.ProgramCacheLen() <= before
	}, 10*time.Second, 10*time.Millisecond)
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
