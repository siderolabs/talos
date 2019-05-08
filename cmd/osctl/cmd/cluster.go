/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"sync"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/cmd/cluster/pkg/node"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/pkg/version"
	"github.com/talos-systems/talos/pkg/userdata/generate"
)

var (
	clusterName string
	image       string
	workers     int
)

// clusterCmd represents the cluster command
var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "A collection of commands for managing local docker-based clusters",
	Long:  ``,
}

// clusterUpCmd represents the cluster up command
var clusterUpCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates a local docker-based kubernetes cluster",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if err := create(); err != nil {
			helpers.Fatalf("%+v", err)
		}
	},
}

// clusterDownCmd represents the cluster up command
var clusterDownCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroys a local docker-based kubernetes cluster",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if err := destroy(); err != nil {
			helpers.Fatalf("%+v", err)
		}
	},
}

// nolint: gocyclo
func create() (err error) {
	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	// Ensure the image is present.

	if err = ensureImageExists(ctx, cli, image); err != nil {
		return err
	}

	// Generate all PKI and tokens required by Talos.

	fmt.Println("generating PKI and tokens")

	ips := []string{"10.5.0.2", "10.5.0.3", "10.5.0.4", "10.5.0.5"}

	input, err := generate.NewInput(clusterName, ips)
	if err != nil {
		return err
	}

	// Setup the network.

	fmt.Println("creating network", clusterName)

	if _, err = createNetwork(cli); err != nil {
		return err
	}

	// Create the master nodes.

	requests := []*node.Request{
		{
			Type:  generate.TypeInit,
			Input: input,
			Image: image,
			Name:  "master-1",
			IP:    net.ParseIP(ips[0]),
		},
		{
			Type:  generate.TypeControlPlane,
			Input: input,
			Image: image,
			Name:  "master-2",
			IP:    net.ParseIP(ips[1]),
		},
		{
			Type:  generate.TypeControlPlane,
			Input: input,
			Image: image,
			Name:  "master-3",
			IP:    net.ParseIP(ips[2]),
		},
	}

	if err := createNodes(requests); err != nil {
		return err
	}

	// Create the worker nodes.

	requests = []*node.Request{}
	for i := 0; i < workers; i++ {
		r := &node.Request{
			Type:  generate.TypeJoin,
			Input: input,
			Image: image,
			Name:  fmt.Sprintf("worker-%d", i),
		}
		requests = append(requests, r)
	}

	if err := createNodes(requests); err != nil {
		return err
	}

	// Create and save the osctl configuration file.

	return saveConfig(input)
}

// nolint: gocyclo
func createNodes(requests []*node.Request) (err error) {
	var wg sync.WaitGroup
	wg.Add(len(requests))

	for _, req := range requests {
		go func(req *node.Request) {
			fmt.Println("creating node", req.Name)

			if err = node.NewNode(clusterName, req); err != nil {
				helpers.Fatalf("failed to create node: %v", err)
			}

			wg.Done()
		}(req)
	}

	wg.Wait()

	return nil
}

func destroy() error {
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	filters := filters.NewArgs()
	filters.Add("label", "talos.owned=true")
	filters.Add("label", "talos.cluster.name="+clusterName)
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true, Filters: filters})
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(len(containers))

	for _, container := range containers {
		go func(container types.Container) {
			fmt.Println("destroying node", container.Names[0][1:])

			err := cli.ContainerRemove(context.Background(), container.ID, types.ContainerRemoveOptions{RemoveVolumes: true, Force: true})
			if err != nil {
				helpers.Fatalf("%+v", err)
			}

			wg.Done()
		}(container)
	}

	wg.Wait()

	fmt.Println("destroying network", clusterName)

	return destroyNetwork(cli)
}

func ensureImageExists(ctx context.Context, cli *client.Client, image string) error {
	// In order to pull an image, the reference must be in canononical
	// format (e.g. domain/repo/image:tag).
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return err
	}

	image = ref.String()

	// To filter the images, we need a familiar name and a tag
	// (e.g. domain/repo/image:tag => repo/image:tag).
	familiarName := reference.FamiliarName(ref)
	tag := ""
	if tagged, isTagged := ref.(reference.Tagged); isTagged {
		tag = tagged.Tag()
	}

	filters := filters.NewArgs()
	filters.Add("reference", familiarName+":"+tag)

	images, err := cli.ImageList(ctx, types.ImageListOptions{Filters: filters})
	if err != nil {
		return err
	}

	if len(images) == 0 {
		fmt.Println("downloading", image)
		var reader io.ReadCloser
		if reader, err = cli.ImagePull(ctx, image, types.ImagePullOptions{}); err != nil {
			return err
		}
		if _, err = io.Copy(ioutil.Discard, reader); err != nil {
			return err
		}
	}

	return nil
}
func createNetwork(cli *client.Client) (types.NetworkCreateResponse, error) {
	options := types.NetworkCreate{
		Labels: map[string]string{
			"talos.owned":        "true",
			"talos.cluster.name": clusterName,
		},
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{
				{
					Subnet: "10.5.0.0/24",
				},
			},
		},
	}

	return cli.NetworkCreate(context.Background(), clusterName, options)
}

func destroyNetwork(cli *client.Client) error {
	filters := filters.NewArgs()
	filters.Add("label", "talos.owned=true")
	filters.Add("label", "talos.cluster.name="+clusterName)

	options := types.NetworkListOptions{
		Filters: filters,
	}

	networks, err := cli.NetworkList(context.Background(), options)
	if err != nil {
		return err
	}

	var result *multierror.Error
	for _, network := range networks {
		if err := cli.NetworkRemove(context.Background(), network.ID); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}

func saveConfig(input *generate.Input) (err error) {
	var talosconfigString string
	if talosconfigString, err = generate.Talosconfig(input); err != nil {
		return err
	}
	newConfig, err := config.FromString(talosconfigString)
	if err != nil {
		return err
	}

	newConfig.Contexts[clusterName].Target = "127.0.0.1"

	c, err := config.Open(talosconfig)
	if err != nil {
		return err
	}
	if c.Contexts == nil {
		c.Contexts = map[string]*config.Context{}
	}
	c.Contexts[clusterName] = newConfig.Contexts[clusterName]

	c.Context = clusterName

	return c.Save(talosconfig)
}

func init() {
	clusterUpCmd.Flags().StringVar(&image, "image", "docker.io/autonomy/talos:"+version.Tag, "the image to use")
	clusterCmd.PersistentFlags().StringVar(&clusterName, "name", "talos_default", "the name of the cluster")
	clusterCmd.PersistentFlags().IntVar(&workers, "workers", 1, "the number of workers to create")
	clusterCmd.AddCommand(clusterUpCmd)
	clusterCmd.AddCommand(clusterDownCmd)
	rootCmd.AddCommand(clusterCmd)
}
