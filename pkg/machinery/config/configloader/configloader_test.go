// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configloader_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
)

// callMethods calls obj's "getter" methods recursively and fails on panic.
//
//nolint:gocyclo
func callMethods(t testing.TB, obj reflect.Value, chain ...string) {
	t.Helper()

	if (obj.Kind() == reflect.Interface || obj.Kind() == reflect.Pointer) && obj.IsNil() {
		return
	}

	typ := obj.Type()

	for i := range obj.NumMethod() {
		method := obj.Method(i)

		if method.Type().NumIn() != 0 {
			continue
		}

		methodName := typ.Method(i).Name
		nextChain := make([]string, len(chain)+1)
		copy(nextChain, chain)
		nextChain[len(nextChain)-1] = methodName
		// t.Log(nextChain)

		// skip known broken methods
		switch methodName {
		case "GetRSAKey", "GetEd25519Key", "GetECDSAKey", "GetCert", "GetKey":
			fallthrough
		case "MarshalYAML":
			fallthrough
		case "Doc":
			fallthrough
		case "APIUrl":
			fallthrough
		case "Endpoint":
			// t.Logf("Skipping %v", nextChain)
			continue
		}

		var resS []reflect.Value

		require.NotPanics(t, func() { resS = method.Call(nil) }, "Method chain: %v", nextChain)

		if len(resS) == 0 {
			continue
		}

		res := resS[0]

		// skip result if it has the same type
		// to avoid infinite recursion on methods like DeepCopy
		if res.Type() == typ {
			continue
		}

		callMethods(t, res, nextChain...)
	}
}

func testConfigLoaderBytes(t testing.TB, b []byte, failOnError bool) {
	t.Helper()

	p, err := configloader.NewFromBytes(b)
	if err != nil {
		if failOnError {
			t.Fatalf("Failed to load: %s.", err)
		} else {
			t.Skipf("Failed to load, skipping: %s.", err)
		}
	}

	callMethods(t, reflect.ValueOf(p))
}

// TODO(aleksi): maybe remove once Go 1.18 is out; see https://github.com/golang/go/issues/47413
func TestConfigLoader(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("testdata", "*.test"))
	require.NoError(t, err)

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			b, err := os.ReadFile(file)
			require.NoError(t, err)

			testConfigLoaderBytes(t, b, true)
		})
	}
}
