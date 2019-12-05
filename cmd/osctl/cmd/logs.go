// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"

	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/talos-systems/talos/api/common"
	"github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/pkg/constants"
)

var follow bool

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs <id>",
	Short: "Retrieve logs for a process or container",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			var namespace string
			if kubernetes {
				namespace = criconstants.K8sContainerdNamespace
			} else {
				namespace = constants.SystemContainerdNamespace
			}
			driver := common.ContainerDriver_CONTAINERD
			if useCRI {
				driver = common.ContainerDriver_CRI
			}

			stream, err := c.Logs(globalCtx, namespace, driver, args[0], follow)
			if err != nil {
				helpers.Fatalf("error fetching logs: %s", err)
			}

			defaultNode := remotePeer(stream.Context())

			respCh, errCh := newLineSlicer(stream)

			for data := range respCh {
				if data.Metadata != nil && data.Metadata.Error != "" {
					_, err = fmt.Fprintf(os.Stderr, "ERROR: %s\n", data.Metadata.Error)
					helpers.Should(err)
					continue
				}

				node := defaultNode
				if data.Metadata != nil && data.Metadata.Hostname != "" {
					node = data.Metadata.Hostname
				}

				_, err = fmt.Printf("%s: %s\n", node, data.Bytes)
				helpers.Should(err)
			}

			if err = <-errCh; err != nil {
				helpers.Fatalf("error getting logs: %v", err)
			}
		})
	},
}

// lineSlicer splits random chunks of bytes coming from nodes into a stream
// of lines aggregated per node.
type lineSlicer struct {
	respCh chan *common.DataResponse
	errCh  chan error
	pipes  map[string]*io.PipeWriter
	wg     sync.WaitGroup
}

func newLineSlicer(stream machine.Machine_LogsClient) (chan *common.DataResponse, chan error) {
	slicer := &lineSlicer{
		respCh: make(chan *common.DataResponse),
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
		slicer.respCh <- &common.DataResponse{
			Metadata: &common.ResponseMetadata{
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
		_ = p.Close() //nolint: errcheck
	}

	slicer.wg.Wait()
}

func (slicer *lineSlicer) run(stream machine.Machine_LogsClient) {
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
		helpers.Should(err)
	}
}

func init() {
	logsCmd.Flags().BoolVarP(&kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")
	logsCmd.Flags().BoolVarP(&useCRI, "use-cri", "c", false, "use the CRI driver")
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "specify if the logs should be streamed")
	rootCmd.AddCommand(logsCmd)
}
