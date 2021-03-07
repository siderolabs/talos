// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

var (
	follow    bool
	tailLines int32
)

// logsCmd represents the logs command.
var logsCmd = &cobra.Command{
	Use:   "logs <service name>",
	Short: "Retrieve logs for a service",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var (
				namespace string
				driver    common.ContainerDriver
			)

			if kubernetes {
				namespace = criconstants.K8sContainerdNamespace
				driver = common.ContainerDriver_CRI
			} else {
				namespace = constants.SystemContainerdNamespace
				driver = common.ContainerDriver_CONTAINERD
			}

			stream, err := c.Logs(ctx, namespace, driver, args[0], follow, tailLines)
			if err != nil {
				return fmt.Errorf("error fetching logs: %s", err)
			}

			defaultNode := client.RemotePeer(stream.Context())

			respCh, errCh := newLineSlicer(stream)

			for data := range respCh {
				if data.Metadata != nil && data.Metadata.Error != "" {
					_, err = fmt.Fprintf(os.Stderr, "ERROR: %s\n", data.Metadata.Error)
					if err != nil {
						return err
					}

					continue
				}

				node := defaultNode
				if data.Metadata != nil && data.Metadata.Hostname != "" {
					node = data.Metadata.Hostname
				}

				_, err = fmt.Printf("%s: %s\n", node, data.Bytes)
				if err != nil {
					return err
				}
			}

			if err = <-errCh; err != nil {
				return fmt.Errorf("error getting logs: %v", err)
			}

			return nil
		})
	},
}

// lineSlicer splits random chunks of bytes coming from nodes into a stream
// of lines aggregated per node.
type lineSlicer struct {
	respCh chan *common.Data
	errCh  chan error
	pipes  map[string]*io.PipeWriter
	wg     sync.WaitGroup
}

func newLineSlicer(stream machine.MachineService_LogsClient) (chan *common.Data, chan error) {
	slicer := &lineSlicer{
		respCh: make(chan *common.Data),
		errCh:  make(chan error, 1),
		pipes:  map[string]*io.PipeWriter{},
	}

	go slicer.run(stream)

	return slicer.respCh, slicer.errCh
}

func (slicer *lineSlicer) chopper(r io.Reader, hostname string) {
	defer slicer.wg.Done()

	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		slicer.respCh <- &common.Data{
			Metadata: &common.Metadata{
				Hostname: hostname,
			},
			Bytes: scanner.Bytes(),
		}
	}
}

func (slicer *lineSlicer) getPipe(node string) *io.PipeWriter {
	pipe, ok := slicer.pipes[node]
	if !ok {
		var piper *io.PipeReader
		piper, pipe = io.Pipe()

		slicer.wg.Add(1)

		go slicer.chopper(piper, node)

		slicer.pipes[node] = pipe
	}

	return pipe
}

func (slicer *lineSlicer) cleanupChoppers() {
	for _, p := range slicer.pipes {
		_ = p.Close() //nolint:errcheck
	}

	slicer.wg.Wait()
}

func (slicer *lineSlicer) run(stream machine.MachineService_LogsClient) {
	defer close(slicer.errCh)
	defer close(slicer.respCh)

	defer slicer.cleanupChoppers()

	for {
		data, err := stream.Recv()
		if err != nil {
			if err == io.EOF || status.Code(err) == codes.Canceled {
				return
			}
			slicer.errCh <- err

			return
		}

		if data.Metadata != nil && data.Metadata.Error != "" {
			// errors are delivered OOB
			slicer.respCh <- data

			continue
		}

		node := ""

		if data.Metadata != nil {
			node = data.Metadata.Hostname
		}

		_, err = slicer.getPipe(node).Write(data.Bytes)
		cli.Should(err)
	}
}

func init() {
	logsCmd.Flags().BoolVarP(&kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "specify if the logs should be streamed")
	logsCmd.Flags().Int32VarP(&tailLines, "tail", "", -1, "lines of log file to display (default is to show from the beginning)")

	logsCmd.Flags().BoolP("use-cri", "c", false, "use the CRI driver")
	logsCmd.Flags().MarkHidden("use-cri") //nolint:errcheck

	addCommand(logsCmd)
}
