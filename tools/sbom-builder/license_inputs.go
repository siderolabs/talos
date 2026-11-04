// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
	modzip "golang.org/x/mod/zip"
)

const goProxyBaseURL = "https://proxy.golang.org"

type licenseModule struct {
	module.Version
	Sum string
}

type localReplacement struct {
	Path, Directory string
}

// licenseClosure mirrors the manifest-only parser in the pinned Syft version:
// replacements are applied in order (including unmatched replacements), then
// exclusions remove paths regardless of version. This is not Go's build list
// or MVS: the SBOM catalogs the manifest, not only packages compiled by Talos.
//
//nolint:gocyclo
func licenseClosure(modBytes, sumBytes []byte) ([]licenseModule, []localReplacement, error) {
	f, err := modfile.Parse("go.mod", modBytes, nil)
	if err != nil {
		return nil, nil, err
	}

	sums := map[module.Version]string{}

	for line := range strings.SplitSeq(string(sumBytes), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		if len(fields) != 3 || !validModuleSum(fields[2]) {
			return nil, nil, fmt.Errorf("malformed go.sum entry")
		}

		key := module.Version{Path: fields[0], Version: fields[1]}
		if previous, ok := sums[key]; ok && previous != fields[2] {
			return nil, nil, fmt.Errorf("conflicting go.sum entry for %s", key)
		}

		sums[key] = fields[2]
	}

	effective := map[string]module.Version{}
	for _, required := range f.Require {
		effective[required.Mod.Path] = required.Mod
	}

	for _, replacement := range f.Replace {
		key := replacement.New.Path
		if replacement.New.Version == "" {
			key = replacement.Old.Path
		} else {
			delete(effective, replacement.Old.Path)
		}

		effective[key] = replacement.New
	}

	for _, excluded := range f.Exclude {
		delete(effective, excluded.Mod.Path)
	}

	keys := make([]string, 0, len(effective))
	for key := range effective {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	var (
		modules []licenseModule
		locals  []localReplacement
	)

	for _, key := range keys {
		m := effective[key]
		if m.Version == "" {
			locals = append(locals, localReplacement{Path: key, Directory: m.Path})

			continue
		}

		if err := module.Check(m.Path, m.Version); err != nil || module.CanonicalVersion(m.Version) != m.Version {
			return nil, nil, fmt.Errorf("invalid pinned module %s", m)
		}

		sum, ok := sums[m]
		if !ok {
			// Without a full go.sum checksum the build never downloads this
			// module, for example the target of a stale version-specific
			// replacement. It gets no license input, so every build reports
			// it the same way.
			continue
		}

		modules = append(modules, licenseModule{Version: m, Sum: sum})
	}

	return modules, locals, nil
}

func validModuleSum(sum string) bool {
	if !strings.HasPrefix(sum, "h1:") {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sum, "h1:"))

	return err == nil && len(decoded) == sha256.Size && "h1:"+base64.StdEncoding.EncodeToString(decoded) == sum
}

func inputZipName(m module.Version) string {
	return fmt.Sprintf("%x.zip", sha256.Sum256([]byte(m.String())))
}

// readInput rejects links and special files, including a replacement between
// lstat and open. Archive bytes are subsequently copied into a private holding
// directory before checksum verification and extraction.
func readInput(name string, limit int64) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("invalid input file %s", name)
	}

	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("input identity changed: %s", name)
	}

	contents, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}

	if int64(len(contents)) > limit {
		return nil, fmt.Errorf("input exceeds size limit: %s", name)
	}

	return contents, nil
}

func moduleManifests(dir string) ([]byte, []byte, error) {
	modBytes, err := readInput(filepath.Join(dir, "go.mod"), modzip.MaxGoMod)
	if err != nil {
		return nil, nil, err
	}

	sumBytes, err := readInput(filepath.Join(dir, "go.sum"), 16<<20)

	return modBytes, sumBytes, err
}

// Only the shape used by all four Dockerfile SBOM consumers is admitted. In
// particular, disabling Go source analysis must never silently omit source,
// nested manifests or binary inputs from a future caller's catalog.
//
//nolint:gocyclo
func validateManifestScan(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		if name != "go.mod" && name != "go.sum" && !strings.HasSuffix(name, ".spdx.json") {
			return fmt.Errorf("manifest-only scan rejects %s", name)
		}

		contents, err := readInput(filepath.Join(dir, name), 64<<20)
		if err != nil {
			return err
		}

		if strings.HasSuffix(name, ".spdx.json") {
			var header struct {
				Version string `json:"spdxVersion"`
				ID      string `json:"SPDXID"`
			}
			if json.Unmarshal(contents, &header) != nil || !strings.HasPrefix(header.Version, "SPDX-") || header.ID != "SPDXRef-DOCUMENT" {
				return fmt.Errorf("invalid SPDX input %s", name)
			}
		}
	}

	return nil
}

type moduleFetcher func(context.Context, module.Version, io.Writer) error

//nolint:gocyclo
func prepareLicenseInputs(ctx context.Context, sourceDir, outputDir string, fetch moduleFetcher) (err error) {
	modBytes, sumBytes, err := moduleManifests(sourceDir)
	if err != nil {
		return err
	}

	modules, locals, err := licenseClosure(modBytes, sumBytes)
	if err != nil {
		return err
	}
	// Local replacement declarations are bound by go.mod, but not downloaded or
	// synthesized as cache entries. This preserves Syft's blank-version identity
	// and existing lack of an external-cache license for these source modules.
	root, err := os.OpenRoot(sourceDir)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck

	for _, local := range locals {
		if filepath.IsAbs(local.Directory) {
			return fmt.Errorf("local replacement must be source-relative: %s", local.Path)
		}

		localMod, err := root.ReadFile(filepath.Join(local.Directory, "go.mod"))
		if err != nil || modfile.ModulePath(localMod) != local.Path {
			return fmt.Errorf("missing or foreign local replacement: %s", local.Path)
		}
	}
	// No fetch or output is allowed until the complete checksum/local-input
	// closure has passed. Never reuse or overwrite an existing output directory.
	if err = ctx.Err(); err != nil {
		return err
	}

	if err = os.Mkdir(outputDir, 0o700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(outputDir) //nolint:errcheck // only the new directory owned by this call
		}
	}()

	if err = os.Chmod(outputDir, 0o700); err != nil {
		return err
	}

	for _, m := range modules {
		if err = ctx.Err(); err != nil {
			return err
		}

		name := filepath.Join(outputDir, inputZipName(m.Version))

		f, createErr := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr != nil {
			return createErr
		}

		fetchErr := fetch(ctx, m.Version, f)
		closeErr := f.Close()

		if fetchErr != nil {
			return fetchErr
		}

		if closeErr != nil {
			return closeErr
		}

		if err = verifyModuleZip(name, m); err != nil {
			return err
		}
	}
	// These exact manifests are the completion marker, not a second list of
	// dependency versions. Consumers derive the closure again from their input.
	if err = ctx.Err(); err != nil {
		return err
	}

	if err = os.WriteFile(filepath.Join(outputDir, "go.sum"), sumBytes, 0o600); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(outputDir, "go.mod"), modBytes, 0o600)
}

func verifyModuleZip(name string, m licenseModule) error {
	if _, err := modzip.CheckZip(m.Version, name); err != nil {
		return err
	}

	z, err := zip.OpenReader(name)
	if err != nil {
		return err
	}
	defer z.Close() //nolint:errcheck

	for _, f := range z.File {
		if !f.Mode().IsRegular() && !f.Mode().IsDir() {
			return fmt.Errorf("special file in module archive %s", m.Version)
		}
	}

	sum, err := dirhash.HashZip(name, dirhash.Hash1)
	if err != nil {
		return err
	}

	if sum != m.Sum {
		return fmt.Errorf("module checksum mismatch: %s", m.Version)
	}

	return nil
}

// materializeLicenseInputs returns a fresh cache, never the mutable compiler
// cache. Revalidate private archive copies before extracting; no extracted
// directory supplied by a caller is trusted or scanned.
//
//nolint:gocyclo
func materializeLicenseInputs(sourceDir, inputsDir string) (string, func(), error) {
	if inputsDir == "" {
		return "", nil, fmt.Errorf("license-inputs is required")
	}

	if err := validateManifestScan(sourceDir); err != nil {
		return "", nil, err
	}

	modBytes, sumBytes, err := moduleManifests(sourceDir)
	if err != nil {
		return "", nil, err
	}

	preparedMod, preparedSum, err := moduleManifests(inputsDir)
	if err != nil {
		return "", nil, err
	}

	if !bytes.Equal(modBytes, preparedMod) || !bytes.Equal(sumBytes, preparedSum) {
		return "", nil, fmt.Errorf("prepared license manifests do not match scan inputs")
	}

	modules, _, err := licenseClosure(modBytes, sumBytes)
	if err != nil {
		return "", nil, err
	}

	entries, err := os.ReadDir(inputsDir)
	if err != nil || len(entries) != len(modules)+2 {
		return "", nil, fmt.Errorf("unexpected license input inventory")
	}

	tmp, err := os.MkdirTemp("", "sbom-licenses-")
	if err != nil {
		return "", nil, err
	}

	cleanup := func() { os.RemoveAll(tmp) } //nolint:errcheck

	fail := func(err error) (string, func(), error) {
		cleanup()

		return "", nil, err
	}
	if err = os.Chmod(tmp, 0o700); err != nil {
		return fail(err)
	}

	cache := filepath.Join(tmp, "mod")
	if err = os.Mkdir(cache, 0o700); err != nil {
		return fail(err)
	}

	for _, m := range modules {
		name := inputZipName(m.Version)

		archive, err := readInput(filepath.Join(inputsDir, name), modzip.MaxZipFile)
		if err != nil {
			return fail(err)
		}

		held := filepath.Join(tmp, name)
		if err = os.WriteFile(held, archive, 0o600); err != nil {
			return fail(err)
		}

		if err = verifyModuleZip(held, m); err != nil {
			return fail(err)
		}

		escaped, err := module.EscapePath(m.Path)
		if err != nil {
			return fail(err)
		}
		// Syft's moduleDir escapes the path but retains the version verbatim.
		if err = modzip.Unzip(filepath.Join(cache, escaped+"@"+m.Version.Version), m.Version, held); err != nil {
			return fail(err)
		}
	}

	return cache, cleanup, nil
}

func allowedModuleURL(u *url.URL) bool {
	return u.Scheme == "https" && u.User == nil && (u.Port() == "" || u.Port() == "443") &&
		(u.Hostname() == "proxy.golang.org" || u.Hostname() == "storage.googleapis.com")
}

//nolint:gocyclo
func fetchModule(ctx context.Context, m module.Version, output io.Writer) error {
	escapedPath, err := module.EscapePath(m.Path)
	if err != nil {
		return err
	}

	escapedVersion, err := module.EscapeVersion(m.Version)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, goProxyBaseURL+"/"+escapedPath+"/@v/"+escapedVersion+".zip", nil)
	if err != nil {
		return err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert

	transport.Proxy = nil // no ambient proxy credentials, netrc or Go environment
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || !allowedModuleURL(req.URL) {
				return fmt.Errorf("module redirect outside approved proxy chain")
			}

			return nil
		},
	}

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download module %s failed", m) // never log signed redirect URLs
	}
	defer response.Body.Close() //nolint:errcheck

	if response.StatusCode != http.StatusOK || response.ContentLength > modzip.MaxZipFile {
		return fmt.Errorf("invalid module response for %s", m)
	}

	n, err := io.Copy(output, io.LimitReader(response.Body, modzip.MaxZipFile+1))
	if err != nil || n > modzip.MaxZipFile {
		return fmt.Errorf("incomplete or oversized module response for %s", m)
	}

	return nil
}
