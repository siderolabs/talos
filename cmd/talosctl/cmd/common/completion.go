// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package common

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// CompleteConfigPatch completes the value of a flag which accepts the `@file` config patch syntax.
func CompleteConfigPatch(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if toComplete != "" && !strings.HasPrefix(toComplete, "@") {
		return nil, cobra.ShellCompDirectiveDefault
	}

	dir, base := splitPathPrefix(strings.TrimPrefix(toComplete, "@"))

	completions, hasDir := configPatchCandidates(dir, base)

	directive := cobra.ShellCompDirectiveNoFileComp

	if hasDir {
		directive |= cobra.ShellCompDirectiveNoSpace
	}

	return completions, directive
}

// splitPathPrefix splits path into the directory prefix as typed and the basename prefix.
func splitPathPrefix(path string) (dir, base string) {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx+1], path[idx+1:]
	}

	return "", path
}

// configPatchCandidates lists the entries of dir whose name starts with base, `@`-prefixed.
func configPatchCandidates(dir, base string) (completions []string, hasDir bool) {
	readDir := dir

	if readDir == "" {
		readDir = "."
	}

	entries, err := os.ReadDir(readDir)
	if err != nil {
		return nil, false
	}

	for _, entry := range entries {
		name := entry.Name()

		if !strings.HasPrefix(name, base) {
			continue
		}

		// hidden entries are only offered when explicitly asked for, as the shells do
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}

		if isDir(readDir, entry) {
			hasDir = true

			completions = append(completions, "@"+dir+name+"/")

			continue
		}

		completions = append(completions, "@"+dir+name)
	}

	return completions, hasDir
}

// isDir resolves symlinks, so that a symlinked directory can be descended into as well.
func isDir(readDir string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}

	if entry.Type()&os.ModeSymlink == 0 {
		return false
	}

	info, err := os.Stat(filepath.Join(readDir, entry.Name()))

	return err == nil && info.IsDir()
}
