// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
)

const fixtureLicense = `MIT License

Copyright (c) 2026 Example contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`

func writeFixture(t *testing.T, name string, content []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(name, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixtureZip(t *testing.T, m module.Version, files map[string]string) ([]byte, string) {
	t.Helper()

	var buffer bytes.Buffer

	writer := zip.NewWriter(&buffer)
	for name, contents := range files {
		file, err := writer.Create(m.String() + "/" + name)
		if err != nil {
			t.Fatal(err)
		}

		if _, err = io.WriteString(file, contents); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	name := filepath.Join(t.TempDir(), "module.zip")
	writeFixture(t, name, buffer.Bytes())

	sum, err := dirhash.HashZip(name, dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}

	return buffer.Bytes(), sum
}

func fixtureModule(t *testing.T, path, version, license string) (module.Version, []byte, string) {
	t.Helper()

	m := module.Version{Path: path, Version: version}

	files := map[string]string{"go.mod": "module " + path + "\n", "README.md": "fixture module\n"}
	if license != "" {
		files["LICENSE"] = license
	}

	archive, sum := fixtureZip(t, m, files)

	return m, archive, sum
}

func singleModuleFixture(t *testing.T) (string, module.Version, []byte, string) {
	t.Helper()
	source := t.TempDir()
	m, archive, sum := fixtureModule(t, "example.com/Module", "v1.2.3", fixtureLicense)
	writeFixture(t, filepath.Join(source, "go.mod"), []byte("module example.com/main\nrequire "+m.Path+" "+m.Version+"\n"))
	writeFixture(t, filepath.Join(source, "go.sum"), []byte(m.Path+" "+m.Version+" "+sum+"\n"))

	return source, m, archive, sum
}

func archiveFetcher(archive []byte) moduleFetcher {
	return func(_ context.Context, _ module.Version, output io.Writer) error {
		_, err := output.Write(archive)

		return err
	}
}

func TestLicenseClosureMatchesManifestSemantics(t *testing.T) {
	hash := "h1:" + strings.Repeat("A", 43) + "="

	sums := []byte("example.com/a v1.0.0 " + hash + "\nexample.com/b v2.0.0+incompatible " + hash + "\nexample.com/Unused v1.1.0 " + hash + "\n")
	for _, test := range []struct {
		name, directives string
		modules          []licenseModule
		locals           []localReplacement
	}{
		{"require", "require example.com/a v1.0.0", []licenseModule{{module.Version{Path: "example.com/a", Version: "v1.0.0"}, hash}}, nil},
		{
			"replace",
			"require example.com/a v1.0.0\nreplace example.com/a => example.com/b v2.0.0+incompatible",
			[]licenseModule{{module.Version{Path: "example.com/b", Version: "v2.0.0+incompatible"}, hash}},
			nil,
		},
		{"unmatched replacement", "replace example.com/a v0.9.0 => example.com/Unused v1.1.0", []licenseModule{{module.Version{Path: "example.com/Unused", Version: "v1.1.0"}, hash}}, nil},
		{"replacement without full checksum", "require example.com/a v1.0.0\nreplace example.com/s v0.9.0 => example.com/Stale v1.0.0", []licenseModule{{module.Version{Path: "example.com/a", Version: "v1.0.0"}, hash}}, nil},
		{"exclude by path", "require example.com/a v1.0.0\nexclude example.com/a v0.9.0", nil, nil},
		{"exclude replacement", "replace example.com/a => example.com/b v2.0.0+incompatible\nexclude example.com/b v2.0.0+incompatible", nil, nil},
		{"local replacement", "require example.com/a v1.0.0\nreplace example.com/a => ./local", nil, []localReplacement{{Path: "example.com/a", Directory: "./local"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			modules, locals, err := licenseClosure([]byte("module example.com/main\n"+test.directives+"\n"), sums)
			if err != nil || !reflect.DeepEqual(modules, test.modules) || !reflect.DeepEqual(locals, test.locals) {
				t.Fatalf("closure: modules=%v locals=%v err=%v", modules, locals, err)
			}
		})
	}
}

func TestLicenseInputsRejectUnpinnedBeforeFetch(t *testing.T) {
	for _, test := range []struct{ name, sum string }{
		{"malformed", "invalid\n"},
		{"bad base64", "example.com/Module v1.2.3 h1:invalid\n"},
		{"wrong hash algorithm", "example.com/Module v1.2.3 h2:" + strings.Repeat("A", 43) + "=\n"},
		{"conflict", "example.com/Module v1.2.3 h1:" + strings.Repeat("A", 43) + "=\nexample.com/Module v1.2.3 h1:" + strings.Repeat("B", 42) + "A=\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, _, _, _ := singleModuleFixture(t)
			writeFixture(t, filepath.Join(source, "go.sum"), []byte(test.sum))
			output := filepath.Join(t.TempDir(), "inputs")
			called := false

			err := prepareLicenseInputs(t.Context(), source, output, func(context.Context, module.Version, io.Writer) error {
				called = true

				return nil
			})
			if err == nil || called {
				t.Fatalf("unpinned input accepted or fetched: err=%v called=%v", err, called)
			}

			if _, err = os.Lstat(output); !os.IsNotExist(err) {
				t.Fatal("failed preflight created output")
			}
		})
	}
}

func TestLicenseInputsSkipModulesWithoutFullChecksum(t *testing.T) {
	for _, test := range []struct{ name, sum string }{
		{"missing", ""},
		{"mod hash only", "example.com/Module v1.2.3/go.mod h1:" + strings.Repeat("A", 43) + "=\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, _, _, _ := singleModuleFixture(t)
			writeFixture(t, filepath.Join(source, "go.sum"), []byte(test.sum))
			output := filepath.Join(t.TempDir(), "inputs")
			called := false

			err := prepareLicenseInputs(t.Context(), source, output, func(context.Context, module.Version, io.Writer) error {
				called = true

				return nil
			})
			if err != nil || called {
				t.Fatalf("module without a full checksum failed or was fetched: err=%v called=%v", err, called)
			}

			entries, err := os.ReadDir(output)
			if err != nil {
				t.Fatal(err)
			}

			names := make([]string, 0, len(entries))
			for _, entry := range entries {
				names = append(names, entry.Name())
			}

			if !reflect.DeepEqual(names, []string{"go.mod", "go.sum"}) {
				t.Fatalf("unexpected license inputs: %v", names)
			}
		})
	}
}

//nolint:gocyclo
func TestLicenseInputsRoundTrip(t *testing.T) {
	source, m, archive, sum := singleModuleFixture(t)

	output := filepath.Join(t.TempDir(), "inputs")
	if err := prepareLicenseInputs(t.Context(), source, output, archiveFetcher(archive)); err != nil {
		t.Fatal(err)
	}

	cache, cleanup, err := materializeLicenseInputs(source, output)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	escaped, err := module.EscapePath(m.Path)
	if err != nil {
		t.Fatal(err)
	}

	moduleDir := filepath.Join(cache, escaped+"@"+m.Version)

	actual, err := dirhash.HashDir(moduleDir, m.String(), dirhash.Hash1)
	if err != nil || actual != sum {
		t.Fatalf("extracted checksum: %s %v", actual, err)
	}

	license, err := os.ReadFile(filepath.Join(moduleDir, "LICENSE"))
	if err != nil || string(license) != fixtureLicense {
		t.Fatal("verified license lost")
	}

	if err = os.WriteFile(filepath.Join(output, inputZipName(m)), []byte("later mutation"), 0o600); err != nil {
		t.Fatal(err)
	}

	license, err = os.ReadFile(filepath.Join(moduleDir, "LICENSE"))
	if err != nil || string(license) != fixtureLicense {
		t.Fatal("post-verification source mutation affected private cache")
	}

	cleanup()

	if _, err = os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal("private cache not removed")
	}
}

//nolint:gocyclo
func TestLicenseInputsRejectCorruptOrMissingArchives(t *testing.T) {
	for _, mode := range []string{"missing", "bytes", "symlink", "directory", "foreign inventory", "changed manifest"} {
		t.Run(mode, func(t *testing.T) {
			source, m, archive, _ := singleModuleFixture(t)

			output := filepath.Join(t.TempDir(), "inputs")
			if err := prepareLicenseInputs(t.Context(), source, output, archiveFetcher(archive)); err != nil {
				t.Fatal(err)
			}

			name := filepath.Join(output, inputZipName(m))

			switch mode {
			case "missing":
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
			case "bytes":
				writeFixture(t, name, []byte("corrupt"))
			case "symlink":
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}

				external := filepath.Join(t.TempDir(), "module.zip")
				writeFixture(t, external, archive)

				if err := os.Symlink(external, name); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}

				if err := os.Mkdir(name, 0o700); err != nil {
					t.Fatal(err)
				}
			case "foreign inventory":
				writeFixture(t, filepath.Join(output, "foreign.zip"), archive)
			case "changed manifest":
				writeFixture(t, filepath.Join(output, "go.mod"), []byte("module other.example/main\n"))
			}

			cache, cleanup, err := materializeLicenseInputs(source, output)
			if err == nil || cache != "" || cleanup != nil {
				t.Fatalf("invalid inputs produced cache: %s %v", cache, err)
			}
		})
	}
}

//nolint:gocyclo
func TestLicensePreparationFailureHasNoCompletedInputs(t *testing.T) {
	for _, mode := range []string{"fetch", "hash", "invalid zip", "unsafe path", "special file"} {
		t.Run(mode, func(t *testing.T) {
			source, m, archive, _ := singleModuleFixture(t)
			fetch := archiveFetcher(archive)

			switch mode {
			case "fetch":
				fetch = func(context.Context, module.Version, io.Writer) error { return errors.New("synthetic missing input") }
			case "hash":
				_, different, _ := fixtureModule(t, m.Path, m.Version, "different license")
				fetch = archiveFetcher(different)
			case "invalid zip":
				fetch = archiveFetcher([]byte("not a zip"))
			case "unsafe path":
				unsafe, sum := fixtureZip(t, m, map[string]string{"../escape": "invalid"})
				writeFixture(t, filepath.Join(source, "go.sum"), []byte(m.Path+" "+m.Version+" "+sum+"\n"))

				fetch = archiveFetcher(unsafe)
			case "special file":
				var buffer bytes.Buffer

				writer := zip.NewWriter(&buffer)
				header := &zip.FileHeader{Name: m.String() + "/LICENSE"}
				header.SetMode(os.ModeSymlink | 0o777)

				w, err := writer.CreateHeader(header)
				if err != nil {
					t.Fatal(err)
				}

				if _, err = io.WriteString(w, "other"); err != nil {
					t.Fatal(err)
				}

				if err = writer.Close(); err != nil {
					t.Fatal(err)
				}

				fetch = archiveFetcher(buffer.Bytes())
			}

			output := filepath.Join(t.TempDir(), "inputs")
			if err := prepareLicenseInputs(t.Context(), source, output, fetch); err == nil {
				t.Fatal("invalid preparation succeeded")
			}

			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatal("failed preparation retained successful-looking input directory")
			}
		})
	}
}

//nolint:gocyclo
func TestLicenseInputsLocalReplacementNeverFetched(t *testing.T) {
	for _, mode := range []string{"valid", "with own license", "missing", "foreign module", "escape", "absolute"} {
		t.Run(mode, func(t *testing.T) {
			source := t.TempDir()

			local := "./local"
			if mode == "escape" {
				local = "../outside"
			}

			if mode == "absolute" {
				local = t.TempDir()
			}

			writeFixture(t, filepath.Join(source, "go.mod"), []byte("module example.com/main\nreplace example.com/local => "+local+"\n"))
			writeFixture(t, filepath.Join(source, "go.sum"), nil)

			if mode != "missing" {
				name := "example.com/local"
				if mode == "foreign module" {
					name = "example.com/foreign"
				}

				writeFixture(t, filepath.Join(source, "local", "go.mod"), []byte("module "+name+"\n"))
			}

			if mode == "with own license" {
				writeFixture(t, filepath.Join(source, "local", "LICENSE"), []byte(fixtureLicense))
			}

			called := false
			output := filepath.Join(t.TempDir(), "inputs")
			err := prepareLicenseInputs(t.Context(), source, output, func(context.Context, module.Version, io.Writer) error {
				called = true

				return nil
			})

			wantSuccess := mode == "valid" || mode == "with own license"
			if called || (err == nil) != wantSuccess {
				t.Fatalf("local replacement: fetched=%v err=%v", called, err)
			}

			if wantSuccess {
				entries, err := os.ReadDir(output)
				if err != nil || len(entries) != 2 {
					t.Fatal("local replacement synthesized cache content")
				}
			}
		})
	}
}

func TestManifestScanRejectsScopeChanges(t *testing.T) {
	for _, name := range []string{"main.go", "nested/go.mod", "nested.spdx.json/go.mod", "binary", "unknown.json", "invalid.spdx.json", "linked.spdx.json"} {
		t.Run(name, func(t *testing.T) {
			source, _, _, _ := singleModuleFixture(t)
			if name == "linked.spdx.json" {
				if err := os.Symlink("go.mod", filepath.Join(source, name)); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFixture(t, filepath.Join(source, name), []byte("not SPDX"))
			}

			if err := validateManifestScan(source); err == nil {
				t.Fatal("manifest-only scope silently widened/narrowed")
			}
		})
	}
}

func TestLicenseInputsDoNotOverwriteOutput(t *testing.T) {
	source, _, archive, _ := singleModuleFixture(t)
	output := t.TempDir()
	writeFixture(t, filepath.Join(output, "owned"), []byte("preserve"))

	if err := prepareLicenseInputs(t.Context(), source, output, archiveFetcher(archive)); err == nil {
		t.Fatal("existing output accepted")
	}

	contents, err := os.ReadFile(filepath.Join(output, "owned"))
	if err != nil || string(contents) != "preserve" {
		t.Fatal("existing output changed")
	}
}

func TestLicensePreparationCanceledHasNoCompletedInputs(t *testing.T) {
	for _, phase := range []string{"before fetch", "during fetch"} {
		t.Run(phase, func(t *testing.T) {
			source, _, archive, _ := singleModuleFixture(t)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			if phase == "before fetch" {
				cancel()
			}

			called := false
			output := filepath.Join(t.TempDir(), "inputs")

			err := prepareLicenseInputs(ctx, source, output, func(_ context.Context, _ module.Version, w io.Writer) error {
				called = true

				cancel()

				_, err := w.Write(archive)

				return err
			})
			if !errors.Is(err, context.Canceled) || called != (phase == "during fetch") {
				t.Fatalf("cancellation ignored: called=%v err=%v", called, err)
			}

			if _, err = os.Stat(output); !os.IsNotExist(err) {
				t.Fatal("canceled preparation retained output")
			}
		})
	}
}

func TestLicenseInputsValidNoLicense(t *testing.T) {
	source, m, _, _ := singleModuleFixture(t)
	_, archive, sum := fixtureModule(t, m.Path, m.Version, "")
	writeFixture(t, filepath.Join(source, "go.sum"), []byte(m.Path+" "+m.Version+" "+sum+"\n"))

	output := filepath.Join(t.TempDir(), "inputs")
	if err := prepareLicenseInputs(t.Context(), source, output, archiveFetcher(archive)); err != nil {
		t.Fatal(err)
	}

	cache, cleanup, err := materializeLicenseInputs(source, output)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	escaped, err := module.EscapePath(m.Path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = os.Stat(filepath.Join(cache, escaped+"@"+m.Version, "LICENSE")); !os.IsNotExist(err) {
		t.Fatal("license fabricated")
	}
}

func TestModuleDownloadURLBoundary(t *testing.T) {
	for _, test := range []struct {
		raw     string
		allowed bool
	}{
		{"https://proxy.golang.org/example.com/a/@v/v1.0.0.zip", true},
		{"https://storage.googleapis.com:443/signed-object?opaque=synthetic", true},
		{"http://proxy.golang.org/module", false},
		{"https://proxy.golang.org:8443/module", false},
		{"https://user:password@proxy.golang.org/module", false},
		{"https://proxy.golang.org.attacker.example/module", false},
		{"file:///tmp/module", false},
		{"https://127.0.0.1/module", false},
	} {
		t.Run(fmt.Sprint(test.allowed, "-", test.raw), func(t *testing.T) {
			u, err := url.Parse(test.raw)
			if err != nil {
				t.Fatal(err)
			}

			if allowedModuleURL(u) != test.allowed {
				t.Fatal("incorrect download authority")
			}
		})
	}
}

func TestActualRootLicenseClosureIsPinned(t *testing.T) {
	modBytes, sumBytes, err := moduleManifests("../..")
	if err != nil {
		t.Fatal(err)
	}

	modules, locals, err := licenseClosure(modBytes, sumBytes)
	if err != nil {
		t.Fatal(err)
	}
	// The module count follows every dependency bump; the closure must only be
	// non-empty and keep the single local machinery replacement.
	if len(modules) == 0 || !reflect.DeepEqual(locals, []localReplacement{{Path: "github.com/siderolabs/talos/pkg/machinery", Directory: "./pkg/machinery"}}) {
		t.Fatalf("unexpected real manifest scope: public=%d local=%v", len(modules), locals)
	}
}
