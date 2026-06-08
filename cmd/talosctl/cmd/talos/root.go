// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/siderolabs/gen/maps"
	_ "github.com/siderolabs/proto-codec/codec" // register codec v2
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// GlobalArgs is the common arguments for the root command.
var GlobalArgs global.Args

// kubernetesNamespaceFlag is embedded into command flag structs that select between
// the system and Kubernetes containerd namespaces via the --kubernetes flag.
type kubernetesNamespaceFlag struct {
	kubernetes bool
}

// useKubernetesNamespace reports whether the Kubernetes containerd namespace is selected.
func (f kubernetesNamespaceFlag) useKubernetesNamespace() bool {
	return f.kubernetes
}

// containerNamespaceFlags is implemented by command flag structs carrying the
// --kubernetes namespace selector; used by the container completion helpers.
type containerNamespaceFlags interface {
	useKubernetesNamespace() bool
}

const pathAutoCompleteLimit = 500

// outputFlushInterval is the quiet period after the last multiplexed response
// before flushing the tabwriter, so partial results appear without thrashing
// column widths while responses are still arriving.
const outputFlushInterval = 500 * time.Millisecond

// NewClientFactory creates a new ClientFactory.
func NewClientFactory(ctx context.Context, flags any, dialOptions ...grpc.DialOption) (*global.ClientFactory, error) {
	return global.NewClientFactory(ctx, &GlobalArgs, flags, dialOptions...)
}

// Commands is a list of commands published by the package.
var Commands []*cobra.Command

func addCommand(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(
		&GlobalArgs.Talosconfig,
		"talosconfig",
		"",
		fmt.Sprintf(
			"the path to the Talos configuration file, defaults to '%s' env variable if set, otherwise '%s' and '%s' in order",
			constants.TalosConfigEnvVar,
			filepath.Join("$HOME", constants.TalosDir, constants.TalosconfigFilename),
			filepath.Join(constants.ServiceAccountMountPath, constants.TalosconfigFilename),
		),
	)
	cmd.PersistentFlags().StringSliceVarP(&GlobalArgs.Nodes, "nodes", "n", []string{}, "target the specified nodes")
	cmd.PersistentFlags().StringSliceVarP(&GlobalArgs.Endpoints, "endpoints", "e", []string{}, "override default endpoints in Talos configuration")
	cli.Should(cmd.RegisterFlagCompletionFunc("nodes", completeNodes))
	cmd.PersistentFlags().StringVarP(&GlobalArgs.Cluster, "cluster", "c", "", "cluster to connect to if a proxy endpoint is used")
	cmd.PersistentFlags().StringVar(&GlobalArgs.CmdContext, "context", "", "context to be used in command")
	cmd.PersistentFlags().StringVar(
		&GlobalArgs.SideroV1KeysDir,
		"siderov1-keys-dir",
		"",
		fmt.Sprintf(
			"the path to the SideroV1 auth PGP keys directory, defaults to '%s' env variable if set, otherwise '%s'; only valid for Contexts that use SideroV1 auth",
			constants.SideroV1KeysDirEnvVar,
			filepath.Join("$HOME", constants.TalosDir, constants.SideroV1KeysDir),
		),
	)
	cli.Should(cmd.RegisterFlagCompletionFunc("context", completeConfigContext))

	Commands = append(Commands, cmd)
}

// completePathFromNode represents tab complete options for `ls` and `ls *` commands.
func completePathFromNode(ctx context.Context, flags any, inputPath string) []string {
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

	paths = getPathFromNode(ctx, flags, pathToSearch, inputPath)

	return maps.Keys(paths)
}

//nolint:gocyclo
func getPathFromNode(ctx context.Context, flags any, path, filter string) map[string]struct{} {
	paths := make(map[string]struct{})

	clientFactory, err := NewClientFactory(ctx, flags)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error creating client factory: %v", err))

		return paths
	}

	defer clientFactory.Close() //nolint:errcheck

	ctx, c, err := clientFactory.BuildClientFirstNode(ctx)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error building client: %v", err))

		return paths
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := c.LS(
		ctx, &machineapi.ListRequest{
			Root: path,
		},
	)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error listing path: %v", err))

		return paths
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF || client.StatusCode(err) == codes.Canceled {
				break
			}

			cobra.CompError(fmt.Sprintf("error streaming results: %s", err))

			break
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

	return paths
}

func getServiceFromNode(ctx context.Context, flags any) []string {
	clientFactory, err := NewClientFactory(ctx, flags)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error creating client factory: %v", err))

		return nil
	}

	defer clientFactory.Close() //nolint:errcheck

	responseChan := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machineapi.ServiceListResponse, error) {
			return c.ServiceList(ctx)
		},
	)

	var svcIDs []string

	for resp := range responseChan {
		if resp.Err != nil {
			cobra.CompError(fmt.Sprintf("error from node %s: %v", resp.Node, resp.Err))

			continue
		}

		for _, msg := range resp.Payload.Messages {
			for _, s := range msg.Services {
				svcIDs = append(svcIDs, s.Id)
			}
		}
	}

	return svcIDs
}

func getContainersFromNode(ctx context.Context, flags containerNamespaceFlags) []string {
	clientFactory, err := NewClientFactory(ctx, flags)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error creating client factory: %v", err))

		return nil
	}

	defer clientFactory.Close() //nolint:errcheck

	kubernetes := flags.useKubernetesNamespace()

	var (
		namespace string
		driver    common.ContainerDriver
	)

	if kubernetes {
		namespace = constants.K8sContainerdNamespace
		driver = common.ContainerDriver_CRI
	} else {
		namespace = constants.SystemContainerdNamespace
		driver = common.ContainerDriver_CONTAINERD
	}

	responseChan := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machineapi.ContainersResponse, error) {
			return c.Containers(ctx, namespace, driver)
		},
	)

	var containerIDs []string

	for resp := range responseChan {
		if resp.Err != nil {
			cobra.CompError(fmt.Sprintf("error from node %s: %v", resp.Node, resp.Err))

			continue
		}

		for _, msg := range resp.Payload.Messages {
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
	}

	return containerIDs
}

func mergeSuggestions(s ...[]string) []string {
	merged := slices.Concat(s...)

	slices.Sort(merged)

	return slices.Compact(merged)
}

func relativeTo(fullPath string, filter string) bool {
	return strings.HasPrefix(fullPath, filter)
}
