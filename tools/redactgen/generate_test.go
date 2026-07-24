// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"flag"
	"os"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update the golden files")

const goldenFile = "testdata/specs.golden"

func TestGenerate(t *testing.T) {
	pkg, err := loadPackage("./testdata/specs")
	if err != nil {
		t.Fatal(err)
	}

	source, err := generate(pkg, options{
		outputFile:  "redact.generated.go",
		header:      "// generated file header\n",
		commandLine: "redactgen -o redact.generated.go .",
	})
	if err != nil {
		t.Fatal(err)
	}

	if *update {
		if err = os.WriteFile(goldenFile, source, 0o644); err != nil {
			t.Fatal(err)
		}

		return
	}

	expected, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatal(err)
	}

	if string(source) != string(expected) {
		t.Fatalf("generated code doesn't match %s, re-run with -update:\n\n%s", goldenFile, source)
	}
}

func TestGenerateErrors(t *testing.T) {
	for _, test := range []struct {
		name        string
		pkgPath     string
		expectedErr string
	}{
		{
			name:        "unsupported type",
			pkgPath:     "./testdata/invalid",
			expectedErr: `UnsupportedSpec.Timeout: redact:"replace" is not supported for type int`,
		},
		{
			name:        "missing DeepCopy",
			pkgPath:     "./testdata/nodeepcopy",
			expectedErr: "type NoCopySpec carries sensitive fields, but has no DeepCopy method",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			pkg, err := loadPackage(test.pkgPath)
			if err != nil {
				t.Fatal(err)
			}

			_, err = generate(pkg, options{outputFile: "redact.generated.go"})
			if err == nil {
				t.Fatal("expected an error")
			}

			if !strings.Contains(err.Error(), test.expectedErr) {
				t.Fatalf("expected the error to contain %q, got %q", test.expectedErr, err)
			}
		})
	}
}
