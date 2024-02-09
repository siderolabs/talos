// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	criconstants "github.com/containerd/containerd/pkg/cri/constants"
	"github.com/siderolabs/gen/maps"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/pkg/cli"
	_ "github.com/siderolabs/talos/pkg/grpc/codec" // register codec
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var kubernetes bool

// GlobalArgs is the common arguments for the root command.
var GlobalArgs global.Args

const pathAutoCompleteLimit = 500

// WithClientNoNodes wraps common code to initialize Talos client and provide cancellable context.
//
// WithClientNoNodes doesn't set any node information on the request context.
func WithClientNoNodes(action func(context.Context, *client.Client) error, dialOptions ...grpc.DialOption) error {
	return GlobalArgs.WithClientNoNodes(action, dialOptions...)
}

// WithClient builds upon WithClientNoNodes to provide set of nodes on request context based on config & flags.
func WithClient(action func(context.Context, *client.Client) error, dialOptions ...grpc.DialOption) error {
	return GlobalArgs.WithClient(action, dialOptions...)
}

// WithClientMaintenance wraps common code to initialize Talos client in maintenance (insecure mode).
func WithClientMaintenance(enforceFingerprints []string, action func(context.Context, *client.Client) error) error {
	return GlobalArgs.WithClientMaintenance(enforceFingerprints, action)
}

// Commands is a list of commands published by the package.
var Commands []*cobra.Command

func addCommand(cmd *cobra.Command) {
	Commands = append(Commands, cmd)
}

// completeResource represents tab complete options for `ls` and `ls *` commands.
func completePathFromNode(inputPath string) []string {
	pathToSearch := inputPath

	// If the pathToSearch is empty, use root '/'
	if pathToSearch == "" {
		pathToSearch = "/"
	}

	var paths map[string]struct{}

	// search up one level to find possible completions
	if pathToSearch != "/" && !strings.HasSuffix(pathToSearch, "/") {
		index := strings.LastIndex(pathToSearch, "/")
		// we need a trailing slash to search for items in a directory
		pathToSearch = pathToSearch[:index] + "/"
	}

	paths = getPathFromNode(pathToSearch, inputPath)

	return maps.Keys(paths)
}

//nolint:gocyclo
func getPathFromNode(path, filter string) map[string]struct{} {
	paths := make(map[string]struct{})

	//nolint:errcheck
	GlobalArgs.WithClient(
		func(ctx context.Context, c *client.Client) error {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			stream, err := c.LS(
				ctx, &machineapi.ListRequest{
					Root: path,
				},
			)
			if err != nil {
				return err
			}

			for {
				resp, err := stream.Recv()
				if err != nil {
					if err == io.EOF || client.StatusCode(err) == codes.Canceled {
						return nil
					}

					return fmt.Errorf("error streaming results: %s", err)
				}

				if resp.Metadata != nil && resp.Metadata.Error != "" {
					continue
				}

				if resp.Error != "" {
					continue
				}

				// skip reference to the same directory
				if resp.RelativeName == "." {
					continue
				}

				// limit the results to a reasonable amount
				if len(paths) > pathAutoCompleteLimit {
					return nil
				}

				// directories have a trailing slash
				if resp.IsDir {
					fullPath := path + resp.RelativeName + "/"

					if relativeTo(fullPath, filter) {
						paths[fullPath] = struct{}{}
					}
				} else {
					fullPath := path + resp.RelativeName

					if relativeTo(fullPath, filter) {
						paths[fullPath] = struct{}{}
					}
				}
			}
		},
	)

	return paths
}

func getServiceFromNode() []string {
	var svcIDs []string

	//nolint:errcheck
	GlobalArgs.WithClient(
		func(ctx context.Context, c *client.Client) error {
			var remotePeer peer.Peer

			resp, err := c.ServiceList(ctx, grpc.Peer(&remotePeer))
			if err != nil {
				return err
			}

			for _, msg := range resp.Messages {
				for _, s := range msg.Services {
					svc := cli.ServiceInfoWrapper{ServiceInfo: s}
					svcIDs = append(svcIDs, svc.Id)
				}
			}

			return nil
		},
	)

	return svcIDs
}

func getContainersFromNode(kubernetes bool) []string {
	var containerIDs []string

	//nolint:errcheck
	GlobalArgs.WithClient(
		func(ctx context.Context, c *client.Client) error {
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

			resp, err := c.Containers(ctx, namespace, driver)
			if err != nil {
				return err
			}

			for _, msg := range resp.Messages {
				for _, p := range msg.Containers {
					if p.Pid == 0 {
						continue
					}

					if kubernetes && p.Id == p.PodId {
						continue
					}

					containerIDs = append(containerIDs, p.Id)
				}
			}

			return nil
		},
	)

	return containerIDs
}

func mergeSuggestions(a, b []string) []string {
	merged := append(slices.Clone(a), b...)

	sort.Strings(merged)

	n := 1

	for i := 1; i < len(merged); i++ {
		if merged[i] != merged[i-1] {
			merged[n] = merged[i]
			n++
		}
	}

	merged = merged[:n]

	return merged
}

func relativeTo(fullPath string, filter string) bool {
	return strings.HasPrefix(fullPath, filter)
}
