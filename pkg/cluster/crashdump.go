// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// APICrashDumper collects crash dump via Talos API.
type APICrashDumper struct {
	ClientProvider
	Info
}

// DefaultServiceLogTailLines specifies number of log lines to tail from each service.
const DefaultServiceLogTailLines = 100

// LogLinesPerService customizes defaults for specific services.
var LogLinesPerService = map[string]int32{
	"etcd": 5000,
}

// CrashDump produces information to help with debugging.
//
// CrashDump implements CrashDumper interface.
//
//nolint:gocyclo
func (s *APICrashDumper) CrashDump(ctx context.Context, out io.Writer) {
	cli, err := s.Client()
	if err != nil {
		fmt.Fprintf(out, "error creating crashdump: %s\n", err)

		return
	}

	for _, node := range s.Nodes() {
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
					logLines, ok := LogLinesPerService[svc.Id]
					if !ok {
						logLines = DefaultServiceLogTailLines
					}

					stream, err := cli.Logs(nodeCtx, constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD, svc.Id, false, logLines)
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

					r.Close() //nolint:errcheck
				}
			}
		}(node)
	}
}
