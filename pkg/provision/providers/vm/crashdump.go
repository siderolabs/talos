// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/talos-systems/talos/pkg/provision"
	"github.com/talos-systems/talos/pkg/tail"
)

// CrashDump produces debug information to help with debugging failures.
func (p *Provisioner) CrashDump(ctx context.Context, cluster provision.Cluster, out io.Writer) {
	state, ok := cluster.(*State)
	if !ok {
		fmt.Fprintf(out, "error inspecting firecracker state, %#+v\n", cluster)

		return
	}

	statePath, err := state.StatePath()
	if err != nil {
		fmt.Fprintf(out, "error getting cluster state path: %s", err)

		return
	}

	logFiles, err := filepath.Glob(filepath.Join(statePath, "*.log"))
	if err != nil {
		fmt.Fprintf(out, "error finding log paths: %s\n", err)

		return
	}

	for _, logFile := range logFiles {
		name := filepath.Base(logFile)

		fmt.Fprintf(out, "%s\n%s\n\n", name, strings.Repeat("=", len(name)))

		f, err := os.Open(logFile)
		if err != nil {
			fmt.Fprintf(out, "error opening file: %s\n", err)

			continue
		}

		if err = tail.SeekLines(f, 5000); err != nil {
			fmt.Fprintf(out, "error seeking to the tail: %s\n", err)
		}

		_, _ = io.Copy(out, f) //nolint:errcheck

		f.Close() //nolint:errcheck
	}
}
