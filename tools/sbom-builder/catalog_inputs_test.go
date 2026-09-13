// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	v2_3 "github.com/spdx/tools-golang/spdx/v2/v2_3"
	"golang.org/x/mod/module"
)

const embeddedSPDXFixture = `{
 "spdxVersion":"SPDX-2.3", "dataLicense":"CC0-1.0", "SPDXID":"SPDXRef-DOCUMENT",
 "name":"fixture-system-package", "documentNamespace":"https://example.com/spdx/fixture",
 "creationInfo":{"created":"2026-01-01T00:00:00Z","creators":["Tool: fixture"]},
 "documentDescribes":["SPDXRef-embedded"],
 "packages":[{"name":"fixture-system-package","SPDXID":"SPDXRef-embedded","versionInfo":"1.0",
 "downloadLocation":"NOASSERTION","filesAnalyzed":false,"licenseConcluded":"Apache-2.0","licenseDeclared":"Apache-2.0","copyrightText":"NOASSERTION"}],
 "files":[{"fileName":"/usr/lib/fixture","SPDXID":"SPDXRef-embedded-file","checksums":[{"algorithm":"SHA256","checksum":"0000000000000000000000000000000000000000000000000000000000000000"}]}],
 "relationships":[{"spdxElementId":"SPDXRef-embedded","relationshipType":"CONTAINS","relatedSpdxElement":"SPDXRef-embedded-file"}]
}`

func readCatalog(t *testing.T, name string) ([]byte, *v2_3.Document) {
	t.Helper()

	contents, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}

	var doc v2_3.Document
	if err = json.Unmarshal(contents, &doc); err != nil {
		t.Fatal(err)
	}

	return contents, &doc
}

func catalogIdentities(doc *v2_3.Document) []string {
	result := make([]string, 0, len(doc.Packages))

	for _, p := range doc.Packages {
		refs := make([]string, 0, len(p.PackageExternalReferences))
		for _, ref := range p.PackageExternalReferences {
			refs = append(refs, ref.Category+":"+ref.RefType+":"+ref.Locator)
		}

		slices.Sort(refs)
		result = append(result, p.PackageName+"@"+p.PackageVersion+"|"+p.PrimaryPackagePurpose+"|"+strings.Join(refs, ","))
	}

	slices.Sort(result)

	return result
}

// This calls the actual pinned Syft cataloger twice: the old production
// configuration (Go source analysis and mutable local cache) and the new
// manifest-only configuration. Synthetic module ZIPs are checksum-pinned;
// neither test mode may use the network or real credentials.
//
//nolint:gocyclo,cyclop
func TestCatalogLicenseInputsAreCacheIndependent(t *testing.T) {
	skipIfFIPS(t)
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	source := t.TempDir()
	archives := map[module.Version][]byte{}

	var sums strings.Builder

	for _, spec := range []struct{ path, version, license string }{
		{"example.com/Module", "v1.2.3", fixtureLicense},
		{"example.com/Fork", "v1.0.0-Beta", fixtureLicense},
		{"example.com/no-license", "v1.0.0", ""},
		{"example.com/unmatched", "v1.0.0", fixtureLicense},
	} {
		m, archive, sum := fixtureModule(t, spec.path, spec.version, spec.license)
		archives[m] = archive
		fmt.Fprintf(&sums, "%s %s %s\n", m.Path, m.Version, sum)
	}

	modBytes := []byte(`module example.com/main
go 1.26.5
require (
 example.com/Module v1.2.3
 example.com/original v1.0.0
 example.com/no-license v1.0.0
 example.com/local v1.0.0
 example.com/excluded v1.0.0
)
replace example.com/original => example.com/Fork v1.0.0-Beta
replace example.com/local => ./local
replace example.com/not-required => example.com/unmatched v1.0.0
exclude example.com/excluded v0.9.0
`)
	writeFixture(t, filepath.Join(source, "go.mod"), modBytes)
	writeFixture(t, filepath.Join(source, "go.sum"), []byte(sums.String()))
	writeFixture(t, filepath.Join(source, "local", "go.mod"), []byte("module example.com/local\n"))
	writeFixture(t, filepath.Join(source, "LICENSE"), []byte("must not be borrowed for local replacement"))

	inputs := filepath.Join(t.TempDir(), "inputs")
	if err := prepareLicenseInputs(t.Context(), source, inputs, func(_ context.Context, m module.Version, output io.Writer) error {
		archive, exists := archives[m]
		if !exists {
			return fmt.Errorf("unexpected fetch %s", m)
		}

		_, err := output.Write(archive)

		return err
	}); err != nil {
		t.Fatal(err)
	}

	// The four Dockerfile owners copy only these manifest/SPDX files, never
	// the source module or local replacement directories, into the scan root.
	scan := t.TempDir()
	writeFixture(t, filepath.Join(scan, "go.mod"), modBytes)
	writeFixture(t, filepath.Join(scan, "go.sum"), []byte(sums.String()))
	writeFixture(t, filepath.Join(scan, "system.spdx.json"), []byte(embeddedSPDXFixture))

	complete, cleanup, err := materializeLicenseInputs(scan, inputs)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	baseline := filepath.Join(t.TempDir(), "baseline.json")
	if err = catalog(scan, "talos", "v1.14.0", "siderolabs", "talos_linux", 1700000000, baseline, complete, true); err != nil {
		t.Fatal(err)
	}

	baselineBytes, baselineDoc := readCatalog(t, baseline)
	baselineIDs := catalogIdentities(baselineDoc)

	names := make([]string, 0, len(baselineDoc.Packages))
	for _, pkg := range baselineDoc.Packages {
		names = append(names, pkg.PackageName)
	}

	for _, name := range []string{"example.com/Module", "example.com/Fork", "example.com/no-license", "example.com/unmatched", "example.com/local", "fixture-system-package"} {
		if !slices.Contains(names, name) {
			t.Fatalf("old catalog lost required identity %s: %v", name, names)
		}
	}

	for _, name := range []string{"example.com/original", "example.com/excluded", "example.com/not-required"} {
		if slices.Contains(names, name) {
			t.Fatalf("old catalog retained replaced/excluded identity %s", name)
		}
	}

	for _, mode := range []string{"empty", "incomplete", "historical", "complete"} {
		t.Run(mode, func(t *testing.T) {
			ambient := t.TempDir()

			switch mode {
			case "incomplete":
				writeFixture(t, filepath.Join(ambient, "example.com", "!module@v1.2.3", "LICENSE"), []byte(fixtureLicense))
			case "historical":
				writeFixture(t, filepath.Join(ambient, "example.com", "!module@v0.9.0", "LICENSE"), []byte(fixtureLicense))
			case "complete":
				if err := os.CopyFS(ambient, os.DirFS(complete)); err != nil {
					t.Fatal(err)
				}
			}

			t.Setenv("GOMODCACHE", ambient)

			oldOutput := filepath.Join(t.TempDir(), "old.json")
			if err := catalog(scan, "talos", "v1.14.0", "siderolabs", "talos_linux", 1700000000, oldOutput, ambient, true); err != nil {
				t.Fatal(err)
			}

			oldBytes, oldDoc := readCatalog(t, oldOutput)
			if !slices.Equal(catalogIdentities(oldDoc), baselineIDs) {
				t.Fatal("old cache changed package identities")
			}

			if mode != "complete" && bytes.Equal(oldBytes, baselineBytes) {
				t.Fatal("old incomplete-cache regression was not reproduced")
			}

			if mode == "complete" && !bytes.Equal(oldBytes, baselineBytes) {
				t.Fatal("old complete-cache baseline unstable")
			}

			newOutput := filepath.Join(t.TempDir(), "new.json")
			if err := run(scan, "talos", "v1.14.0", "siderolabs", "talos_linux", 1700000000, newOutput, inputs); err != nil {
				t.Fatal(err)
			}

			newBytes, newDoc := readCatalog(t, newOutput)
			if !slices.Equal(catalogIdentities(newDoc), baselineIDs) {
				t.Fatal("manifest-only cataloger changed package identities")
			}

			if !bytes.Equal(newBytes, baselineBytes) {
				t.Fatal("verified catalog differs from fully enriched old catalog")
			}

			for _, p := range newDoc.Packages {
				switch p.PackageName {
				case "example.com/Module", "example.com/Fork", "example.com/unmatched":
					if p.PackageLicenseConcluded != "MIT" {
						t.Fatalf("license lost for %s: %s", p.PackageName, p.PackageLicenseConcluded)
					}
				case "example.com/local", "example.com/no-license":
					if p.PackageLicenseConcluded != "NOASSERTION" {
						t.Fatalf("license invented for %s: %s", p.PackageName, p.PackageLicenseConcluded)
					}
				}
			}
		})
	}
}

//nolint:gocyclo
func TestCatalogInputFailureCreatesNoSPDX(t *testing.T) {
	for _, mode := range []string{"absent inputs", "corrupt zip", "source file", "nested manifest", "invalid SPDX", "linked input", "missing checksum"} {
		t.Run(mode, func(t *testing.T) {
			source, m, archive, _ := singleModuleFixture(t)

			inputs := filepath.Join(t.TempDir(), "inputs")
			if err := prepareLicenseInputs(t.Context(), source, inputs, archiveFetcher(archive)); err != nil {
				t.Fatal(err)
			}

			switch mode {
			case "absent inputs":
				inputs = ""
			case "corrupt zip":
				writeFixture(t, filepath.Join(inputs, inputZipName(m)), []byte("corrupt"))
			case "source file":
				writeFixture(t, filepath.Join(source, "main.go"), []byte("package main\n"))
			case "nested manifest":
				writeFixture(t, filepath.Join(source, "nested", "go.mod"), []byte("module example.com/nested\n"))
			case "invalid SPDX":
				writeFixture(t, filepath.Join(source, "broken.spdx.json"), []byte("not SPDX"))
			case "linked input":
				if err := os.Symlink("go.mod", filepath.Join(source, "linked.spdx.json")); err != nil {
					t.Fatal(err)
				}
			case "missing checksum":
				writeFixture(t, filepath.Join(source, "go.sum"), nil)
			}

			output := filepath.Join(t.TempDir(), "result.json")
			if err := run(source, "talos", "v1.14.0", "siderolabs", "talos_linux", 1700000000, output, inputs); err == nil {
				t.Fatal("invalid input accepted")
			}

			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatal("failed closure created SPDX")
			}
		})
	}
}

func TestDockerfileAllSBOMConsumersUsePreparedInputs(t *testing.T) {
	contents, err := os.ReadFile("../../Dockerfile")
	if err != nil {
		t.Fatal(err)
	}

	text := string(contents)
	if strings.Count(text, "--prepare-license-inputs /sbom-license-inputs") != 1 {
		t.Fatal("shared preparation missing or duplicated")
	}

	for _, stage := range []string{"sbom-container-arm64-generate", "sbom-container-amd64-generate", "sbom-arm64-generate", "sbom-amd64-generate"} {
		_, body, exists := strings.Cut(text, "FROM build-sbom AS "+stage+"\n")
		if !exists {
			t.Fatalf("missing shared consumer %s", stage)
		}

		body, _, _ = strings.Cut(body, "\nFROM ")
		if strings.Count(body, "--license-inputs /sbom-license-inputs") != 1 || !strings.Contains(body, "cp go.mod go.sum /tmp/sbom-src/") {
			t.Fatalf("consumer %s does not bind prepared manifests", stage)
		}
	}
}
