// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"
	"io"

	cl "github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/provision"
)

// CrashDump produces debug information to help with debugging failures.
func (p *Provisioner) CrashDump(ctx context.Context, cluster provision.Cluster, out io.Writer) {
	cl.Crashdump(ctx, cluster, out)
}
