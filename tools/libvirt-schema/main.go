// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

const (
	maxArchiveBytes = 64 << 20
	maxSchemaBytes  = 8 << 20
)

func main() {
	version := flag.String("version", "", "libvirt release tag (for example, v12.7.0)")
	url := flag.String("url", "", "libvirt source archive URL (defaults to the release archive)")
	checksum := flag.String("sha256", "", "expected source archive SHA-256")
	output := flag.String("output", "", "schema output directory")

	flag.Parse()

	if *version == "" || *checksum == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "-version, -sha256, and -output are required")
		os.Exit(1)
	}

	release := strings.TrimPrefix(*version, "v")
	if *url == "" {
		*url = "https://download.libvirt.org/libvirt-" + release + ".tar.xz"
	}

	if err := generate(*url, *checksum, *output, release); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(url, checksum, output, version string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create libvirt request: %w", err)
	}

	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return fmt.Errorf("download libvirt source: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download libvirt source: HTTP %d", resp.StatusCode)
	}

	archive, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveBytes+1))
	if err != nil {
		return fmt.Errorf("read libvirt source: %w", err)
	}

	if len(archive) > maxArchiveBytes {
		return errors.New("libvirt source exceeds archive size limit")
	}

	expected, err := hex.DecodeString(checksum)
	if err != nil || len(expected) != sha256.Size {
		return errors.New("invalid libvirt source SHA-256")
	}

	actual := sha256.Sum256(archive)
	if !bytes.Equal(actual[:], expected) {
		return fmt.Errorf("libvirt source checksum mismatch: got %x, want %x", actual, expected)
	}

	files, err := extractSchemas(archive, version)
	if err != nil {
		return err
	}

	return publish(output, files, version, checksum)
}

func extractSchemas(archive []byte, version string) (map[string][]byte, error) {
	all, err := readTarFiles(archive, version)
	if err != nil {
		return nil, err
	}

	selected := make(map[string][]byte)

	for _, name := range []string{"COPYING", "COPYING.LESSER"} {
		data, ok := all[name]
		if !ok {
			return nil, fmt.Errorf("missing libvirt %s", name)
		}

		selected[name] = data
	}

	if err := visitSchema("domain.rng", all, selected); err != nil {
		return nil, err
	}

	return selected, nil
}

func readTarFiles(archive []byte, version string) (map[string][]byte, error) {
	decompressor, err := xz.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("decompress libvirt source: %w", err)
	}

	reader := tar.NewReader(decompressor)
	files := make(map[string][]byte)
	prefix := "libvirt-" + version + "/"

	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("read libvirt tarball: %w", err)
		}

		if err := readTarEntry(reader, header, prefix, files); err != nil {
			return nil, err
		}
	}

	return files, nil
}

func readTarEntry(reader *tar.Reader, header *tar.Header, prefix string, files map[string][]byte) error {
	name, ok := strings.CutPrefix(header.Name, prefix)
	if !ok || header.Typeflag != tar.TypeReg {
		return nil
	}

	if name != "COPYING" && name != "COPYING.LESSER" && !strings.HasPrefix(name, "src/conf/schemas/") {
		return nil
	}

	if header.Size > maxSchemaBytes {
		return fmt.Errorf("libvirt file %q exceeds size limit", name)
	}

	data, err := io.ReadAll(io.LimitReader(reader, maxSchemaBytes+1))
	if err != nil || len(data) > maxSchemaBytes {
		return fmt.Errorf("read libvirt file %q: %w", name, err)
	}

	if _, exists := files[name]; exists {
		return fmt.Errorf("duplicate libvirt file %q", name)
	}

	files[name] = data

	return nil
}

func visitSchema(name string, all, selected map[string][]byte) error {
	if name != path.Base(name) || !strings.HasSuffix(name, ".rng") {
		return fmt.Errorf("unsafe libvirt schema include %q", name)
	}

	if _, ok := selected[name]; ok {
		return nil
	}

	data, ok := all["src/conf/schemas/"+name]
	if !ok {
		return fmt.Errorf("missing libvirt schema include %q", name)
	}

	selected[name] = data

	return visitIncludes(name, data, all, selected)
}

func visitIncludes(name string, data []byte, all, selected map[string][]byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return fmt.Errorf("parse libvirt schema %q: %w", name, err)
		}

		start, ok := token.(xml.StartElement)
		if !ok || !isSchemaReference(start) {
			continue
		}

		if err := visitReference(name, start, all, selected); err != nil {
			return err
		}
	}

	return nil
}

func isSchemaReference(start xml.StartElement) bool {
	return start.Name.Space == "http://relaxng.org/ns/structure/1.0" &&
		(start.Name.Local == "include" || start.Name.Local == "externalRef")
}

func visitReference(name string, start xml.StartElement, all, selected map[string][]byte) error {
	found := false

	for _, attr := range start.Attr {
		if attr.Name.Local != "href" || attr.Name.Space != "" {
			continue
		}

		if err := visitSchema(attr.Value, all, selected); err != nil {
			return err
		}

		found = true
	}

	if !found {
		return fmt.Errorf("libvirt schema %q has include without href", name)
	}

	return nil
}

func publish(output string, files map[string][]byte, version, checksum string) error {
	parent := filepath.Dir(output)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create schema parent: %w", err)
	}

	staging, err := os.MkdirTemp(parent, ".libvirt-schema-*")
	if err != nil {
		return fmt.Errorf("stage libvirt schemas: %w", err)
	}
	defer os.RemoveAll(staging) //nolint:errcheck

	if err := stageFiles(staging, files, version, checksum); err != nil {
		return err
	}

	return replaceOutput(output, staging)
}

func stageFiles(staging string, files map[string][]byte, version, checksum string) error {
	if err := os.Chmod(staging, 0o755); err != nil {
		return fmt.Errorf("set schema directory permissions: %w", err)
	}

	readme := fmt.Sprintf(strings.Join([]string{
		"# libvirt domain schemas\n\n",
		"Generated by `make generate` from libvirt %s.\n",
		"Source archive SHA-256: `%s` (pinned by siderolabs/extensions/hypervisors/vars.yaml).\n",
		"The included RNG files form the transitive domain.rng include closure.\n",
		"COPYING and COPYING.LESSER are the upstream license texts.\n",
		"Update LIBVIRT_VERSION and LIBVIRT_SHA256 in Makefile together, then run `make generate`.\n",
	}, ""), version, checksum)
	files["README.md"] = []byte(readme)

	for name, data := range files {
		if name != filepath.Base(name) {
			return fmt.Errorf("invalid generated filename %q", name)
		}

		if err := os.WriteFile(filepath.Join(staging, name), data, 0o644); err != nil {
			return fmt.Errorf("stage libvirt file %q: %w", name, err)
		}
	}

	return nil
}

func replaceOutput(output, staging string) error {
	backup, err := os.MkdirTemp(filepath.Dir(output), ".libvirt-backup-*")
	if err != nil {
		return fmt.Errorf("reserve schema backup: %w", err)
	}

	if err := os.Remove(backup); err != nil {
		return fmt.Errorf("release schema backup reservation: %w", err)
	}

	if err := os.Rename(output, backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("backup old libvirt schemas: %w", err)
	}

	if err := os.Rename(staging, output); err != nil {
		if restoreErr := os.Rename(backup, output); restoreErr != nil && !errors.Is(restoreErr, os.ErrNotExist) {
			return errors.Join(err, fmt.Errorf("restore old libvirt schemas from %q: %w", backup, restoreErr))
		}

		return fmt.Errorf("publish libvirt schemas: %w", err)
	}

	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove old libvirt schemas from %q: %w", backup, err)
	}

	return nil
}
