// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main builds a SELinux-labeled squashfs image without requiring
// fakeroot or write access to security.* xattrs on the source tree.
//
// It walks the source rootfs, looks up each path's SELinux context against
// the supplied file_contexts, and emits a mksquashfs pseudo-file definition
// list. mksquashfs is then invoked with -xattrs-exclude '.*' so it ignores
// any xattrs on the source filesystem and -pf <pseudo> so it embeds the
// SELinux labels directly into the image.
//
// This is the equivalent, for the rootfs build, of what siderolabs/talos
// PR #13075 did for extension compression.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type fileType int

const (
	typeAny fileType = iota
	typeReg
	typeDir
	typeLnk
	typeChr
	typeBlk
	typeFifo
	typeSock
)

type rule struct {
	re      *regexp.Regexp
	ftype   fileType
	context string
}

func parseTypeSpec(s string) (fileType, error) {
	switch s {
	case "--":
		return typeReg, nil
	case "-d":
		return typeDir, nil
	case "-l":
		return typeLnk, nil
	case "-c":
		return typeChr, nil
	case "-b":
		return typeBlk, nil
	case "-p":
		return typeFifo, nil
	case "-s":
		return typeSock, nil
	}

	return typeAny, fmt.Errorf("unknown file_contexts type spec %q", s)
}

func parseFileContexts(path string) ([]rule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	var rules []rule

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)

		var (
			pattern, ctx string
			ft           = typeAny
		)

		switch len(fields) {
		case 2:
			pattern, ctx = fields[0], fields[1]
		case 3:
			var err error
			if ft, err = parseTypeSpec(fields[1]); err != nil {
				return nil, err
			}

			pattern, ctx = fields[0], fields[2]
		default:
			return nil, fmt.Errorf("malformed file_contexts line: %q", line)
		}

		// file_contexts patterns are anchored against the full path.
		re, err := regexp.Compile("^" + pattern + "$")
		if err != nil {
			return nil, fmt.Errorf("compiling pattern %q: %w", pattern, err)
		}

		rules = append(rules, rule{re: re, ftype: ft, context: ctx})
	}

	return rules, scanner.Err()
}

func fileTypeOf(info fs.FileInfo) fileType {
	m := info.Mode()

	switch {
	case m&fs.ModeSymlink != 0:
		return typeLnk
	case m.IsDir():
		return typeDir
	case m&fs.ModeDevice != 0:
		if m&fs.ModeCharDevice != 0 {
			return typeChr
		}

		return typeBlk
	case m&fs.ModeNamedPipe != 0:
		return typeFifo
	case m&fs.ModeSocket != 0:
		return typeSock
	default:
		return typeReg
	}
}

// lookup returns the SELinux context for path with the given file type.
//
// libselinux's selabel_lookup walks file_contexts in reverse and returns the
// first match — equivalent to "last matching entry wins" — and skips entries
// whose type spec does not match the file type.
func lookup(rules []rule, path string, ft fileType) string {
	for _, r := range slices.Backward(rules) {
		if r.ftype != typeAny && r.ftype != ft {
			continue
		}

		if r.re.MatchString(path) {
			return r.context
		}
	}

	return ""
}

func writePseudo(w *os.File, rootDir string, rules []rule) error {
	bw := bufio.NewWriter(w)

	walkErr := filepath.WalkDir(rootDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(rootDir, p)
		if err != nil {
			return err
		}

		imgPath := "/" + rel
		if rel == "." {
			imgPath = "/"
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		ctx := lookup(rules, imgPath, fileTypeOf(info))
		if ctx == "" {
			return nil
		}

		// libselinux's setfilecon writes the xattr with a trailing NUL byte
		// (lsetxattr len = strlen(ctx)+1). To produce a byte-identical
		// squashfs vs. setfiles+fakeroot, encode the value as 0s<base64>
		// with the NUL terminator included. mksquashfs's pseudo-file syntax
		// supports 0t (text, default), 0x (hex), 0s (base64); the 0x parser
		// in the squashfs-tools shipped with the Talos build env segfaults,
		// so we use base64.
		b64 := base64.StdEncoding.EncodeToString([]byte(ctx + "\x00"))

		_, err = fmt.Fprintf(bw, "%s x security.selinux=0s%s\n", imgPath, b64)

		return err
	})
	if walkErr != nil {
		return walkErr
	}

	return bw.Flush()
}

func run(ctx context.Context) error {
	if len(os.Args) != 5 {
		return fmt.Errorf("usage: %s <root_dir> <output_image> <file_contexts> <compression_level>", os.Args[0])
	}

	rootDir, output, fcPath, level := os.Args[1], os.Args[2], os.Args[3], os.Args[4]

	rules, err := parseFileContexts(fcPath)
	if err != nil {
		return fmt.Errorf("parse file_contexts: %w", err)
	}

	pseudo, err := os.CreateTemp("", "labeled-squashfs-pseudo-*")
	if err != nil {
		return fmt.Errorf("create pseudo file: %w", err)
	}

	defer os.Remove(pseudo.Name()) //nolint:errcheck

	if err := writePseudo(pseudo, rootDir, rules); err != nil {
		pseudo.Close() //nolint:errcheck

		return fmt.Errorf("emit pseudo definitions: %w", err)
	}

	if err := pseudo.Close(); err != nil {
		return fmt.Errorf("close pseudo file: %w", err)
	}

	cmd := exec.CommandContext(
		ctx,
		"mksquashfs",
		rootDir, output,
		"-all-root", "-noappend",
		"-comp", "zstd", "-Xcompression-level", level,
		"-no-progress",
		"-xattrs-exclude", ".*",
		"-pf", pseudo.Name(),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mksquashfs: %w", err)
	}

	return nil
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "labeled-squashfs:", err)
		os.Exit(1)
	}
}
