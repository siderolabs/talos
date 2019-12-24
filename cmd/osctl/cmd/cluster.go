// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
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
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/constants"
	talosnet "github.com/talos-systems/talos/pkg/net"
)

var (
	clusterName   string
	nodeImage     string
	networkCIDR   string
	networkMTU    string
	workers       int
	masters       int
	clusterCpus   string
	clusterMemory int
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
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return create()
	},
}

// clusterDownCmd represents the cluster up command
var clusterDownCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroys a local docker-based kubernetes cluster",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return destroy()
	},
}

// nolint: gocyclo
func create() (err error) {
	ctx := context.Background()

	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	if masters < 1 {
		return fmt.Errorf("number of masters can't be less than 1")
	}

	nanoCPUs, err := parseCPUShare()
	if err != nil {
		return fmt.Errorf("error parsing --cpus: %s", err)
	}

	memory := int64(clusterMemory) * 1024 * 1024

	// Ensure the image is present.

	if err = ensureImageExists(ctx, cli, nodeImage); err != nil {
		return err
	}

	// Validate CIDR range and allocate IPs
	fmt.Println("validating CIDR and reserving master IPs")

	_, cidr, err := net.ParseCIDR(networkCIDR)
	if err != nil {
		return fmt.Errorf("error validating cidr block: %w", err)
	}

	// Set starting ip at 2nd ip in range, ex: 192.168.0.2
	ips := make([]string, masters)

	var masterIP net.IP
	for i := range ips {
		masterIP, err = talosnet.NthIPInNetwork(cidr, i+2)
		if err != nil {
			return err
		}

		ips[i] = masterIP.String()
	}

	// Generate all PKI and tokens required by Talos.
	fmt.Println("generating PKI and tokens")

	input, err := generate.NewInput(clusterName, "https://"+ips[0]+":6443", kubernetesVersion)
	if err != nil {
		return err
	}

	// Setup the network.

	fmt.Println("creating network", clusterName)

	if _, err = createNetwork(cli); err != nil {
		return fmt.Errorf("a cluster might already exist, run \"osctl cluster destroy\" to permanently delete the existing cluster, and try again: %w", err)
	}

	// Create the master nodes.

	requests := make([]*node.Request, masters)
	for i := range requests {
		requests[i] = &node.Request{
			Input:    *input,
			Image:    nodeImage,
			Name:     fmt.Sprintf("%s-master-%d", clusterName, i+1),
			IP:       net.ParseIP(ips[i]),
			Memory:   memory,
			NanoCPUs: nanoCPUs,
		}

		if i == 0 {
			requests[i].Type = generate.TypeInit
		} else {
			requests[i].Type = generate.TypeControlPlane
		}
	}

	if err := createNodes(requests); err != nil {
		return err
	}

	// Create the worker nodes.

	requests = []*node.Request{}

	for i := 1; i <= workers; i++ {
		r := &node.Request{
			Type:     generate.TypeJoin,
			Input:    *input,
			Image:    nodeImage,
			Name:     fmt.Sprintf("%s-worker-%d", clusterName, i),
			Memory:   memory,
			NanoCPUs: nanoCPUs,
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
				helpers.Fatalf("failed to create node: %w", err)
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

		// nolint: errcheck
		defer reader.Close()

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
					Subnet: networkCIDR,
				},
			},
		},
		Options: map[string]string{
			"com.docker.network.driver.mtu": networkMTU,
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
	newConfig := &config.Config{
		Context: input.ClusterName,
		Contexts: map[string]*config.Context{
			input.ClusterName: {
				Endpoints: []string{"127.0.0.1"},
				CA:        base64.StdEncoding.EncodeToString(input.Certs.OS.Crt),
				Crt:       base64.StdEncoding.EncodeToString(input.Certs.Admin.Crt),
				Key:       base64.StdEncoding.EncodeToString(input.Certs.Admin.Key),
			},
		},
	}

	c, err := config.Open(talosconfig)
	if err != nil {
		return err
	}

	if c.Contexts == nil {
		c.Contexts = map[string]*config.Context{}
	}

	c.Contexts[input.ClusterName] = newConfig.Contexts[input.ClusterName]

	c.Context = input.ClusterName

	return c.Save(talosconfig)
}

func parseCPUShare() (int64, error) {
	cpu, ok := new(big.Rat).SetString(clusterCpus)
	if !ok {
		return 0, fmt.Errorf("failed to parsing as a rational number: %s", clusterCpus)
	}

	nano := cpu.Mul(cpu, big.NewRat(1e9, 1))
	if !nano.IsInt() {
		return 0, errors.New("value is too precise")
	}

	return nano.Num().Int64(), nil
}

func init() {
	clusterUpCmd.Flags().StringVar(&nodeImage, "image", defaultImage(constants.DefaultTalosImageRepository), "the image to use")
	clusterUpCmd.Flags().StringVar(&networkMTU, "mtu", "1500", "MTU of the docker bridge network")
	clusterUpCmd.Flags().StringVar(&networkCIDR, "cidr", "10.5.0.0/24", "CIDR of the docker bridge network")
	clusterUpCmd.Flags().IntVar(&workers, "workers", 1, "the number of workers to create")
	clusterUpCmd.Flags().IntVar(&masters, "masters", 1, "the number of masters to create")
	clusterUpCmd.Flags().StringVar(&clusterCpus, "cpus", "1.5", "the share of CPUs as fraction (each container)")
	clusterUpCmd.Flags().IntVar(&clusterMemory, "memory", 1024, "the limit on memory usage in MB (each container)")
	clusterUpCmd.Flags().StringVar(&kubernetesVersion, "kubernetes-version", constants.DefaultKubernetesVersion, "desired kubernetes version to run")
	clusterCmd.PersistentFlags().StringVar(&clusterName, "name", "talos-default", "the name of the cluster")
	clusterCmd.AddCommand(clusterUpCmd)
	clusterCmd.AddCommand(clusterDownCmd)
	rootCmd.AddCommand(clusterCmd)
}
