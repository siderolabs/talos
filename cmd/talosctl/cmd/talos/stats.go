// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// statsCmd represents the stats command.
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Get container stats",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClientAndNodes(cmd.Context(), func(ctx context.Context, c *client.Client, nodes []string) error {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			var (
				namespace string
				driver    common.ContainerDriver
			)

			if kubernetesFlag {
				namespace = constants.K8sContainerdNamespace
				driver = common.ContainerDriver_CRI
			} else {
				namespace = constants.SystemContainerdNamespace
				driver = common.ContainerDriver_CONTAINERD
			}

			responseChan := multiplex.Unary(
				ctx, nodes,
				func(ctx context.Context) (*machineapi.StatsResponse, error) {
					return c.Stats(ctx, namespace, driver)
				},
			)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NODE\tNAMESPACE\tID\tMEMORY(MB)\tCPU")

			flushTimer := time.NewTimer(outputFlushInterval)
			defer flushTimer.Stop()

			flushTimer.Stop()

			var errs error

			for {
				select {
				case resp, ok := <-responseChan:
					if !ok {
						return errors.Join(errs, w.Flush())
					}

					if resp.Err != nil {
						errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))
					} else {
						for _, msg := range resp.Payload.Messages {
							slices.SortFunc(msg.Stats, func(a, b *machineapi.Stat) int { return strings.Compare(a.Id, b.Id) })

							for _, s := range msg.Stats {
								display := s.Id
								if s.Id != s.PodId {
									// container in a sandbox
									display = "└─ " + display
								}

								fmt.Fprintf(w, "%s\t%s\t%s\t%.2f\t%d\n", resp.Node, s.Namespace, display, float64(s.MemoryUsage)*1e-6, s.CpuUsage)
							}
						}
					}

					flushTimer.Reset(outputFlushInterval)
				case <-flushTimer.C:
					if err := w.Flush(); err != nil {
						errs = errors.Join(errs, fmt.Errorf("error flushing output: %w", err))
					}
				}
			}
		})
	},
}

func init() {
	statsCmd.Flags().BoolVarP(&kubernetesFlag, "kubernetes", "k", false, "use the k8s.io containerd namespace")

	statsCmd.Flags().Bool("use-cri", false, "use the CRI driver")
	statsCmd.Flags().MarkHidden("use-cri") //nolint:errcheck

	addCommand(statsCmd)
}
