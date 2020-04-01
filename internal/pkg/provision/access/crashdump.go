// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package access

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/talos-systems/talos/api/common"
	"github.com/talos-systems/talos/pkg/client"
	"github.com/talos-systems/talos/pkg/constants"
)

// CrashDump produces debug information to help with debugging failures.
func (a *adapter) CrashDump(ctx context.Context, out io.Writer) {
	cli, err := a.Client()
	if err != nil {
		fmt.Fprintf(out, "error creating crashdump: %s\n", err)
		return
	}

	for _, node := range a.Info().Nodes {
		func(node string) {
			nodeCtx, nodeCtxCancel := context.WithTimeout(client.WithNodes(ctx, node), 30*time.Second)
			defer nodeCtxCancel()

			fmt.Fprintf(out, "\n%s\n%s\n\n", node, strings.Repeat("=", len(node)))

			services, err := cli.ServiceList(nodeCtx)
			if err != nil {
				fmt.Fprintf(out, "error getting services: %s\n", err)
				return
			}

			for _, msg := range services.Messages {
				for _, svc := range msg.Services {
					stream, err := cli.Logs(nodeCtx, constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD, svc.Id, false, 100)
					if err != nil {
						fmt.Fprintf(out, "error getting service logs for %s: %s\n", svc.Id, err)
						continue
					}

					r, errCh, err := client.ReadStream(stream)
					if err != nil {
						fmt.Fprintf(out, "error getting service logs for %s: %s\n", svc.Id, err)
						continue
					}

					fmt.Fprintf(out, "\n> %s\n%s\n\n", svc.Id, strings.Repeat("-", len(svc.Id)+2))

					_, err = io.Copy(out, r)
					if err != nil {
						fmt.Fprintf(out, "error streaming service logs: %s\n", err)
					}

					err = <-errCh
					if err != nil {
						fmt.Fprintf(out, "error streaming service logs: %s\n", err)
					}

					r.Close() //nolint: errcheck
				}
			}
		}(node.PrivateIP.String())
	}
}
