// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	cl "github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/provision"
)

// CrashDump produces debug information to help with debugging failures.
func (p *Provisioner) CrashDump(ctx context.Context, cluster provision.Cluster, logWriter io.Writer) {
	statePath, err := cluster.StatePath()
	if err != nil {
		fmt.Fprintf(logWriter, "error getting state path: %s\n", err)

		return
	}

	supportZipPath := filepath.Join(statePath, "support.zip")

	cl.Crashdump(ctx, cluster, logWriter, supportZipPath)
}
