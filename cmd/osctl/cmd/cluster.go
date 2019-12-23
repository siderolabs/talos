// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/internal/pkg/provision/access"
	"github.com/talos-systems/talos/internal/pkg/provision/check"
	"github.com/talos-systems/talos/internal/pkg/provision/providers/docker"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/constants"
)

var (
	clusterName   string
	nodeImage     string
	networkCIDR   string
	networkMTU    int
	workers       int
	masters       int
	clusterCpus   string
	clusterMemory int
	clusterWait   bool
)

const baseNetwork = "10.5.0.%d"

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
		return helpers.WithCLIContext(context.Background(), create)
	},
}

// clusterDownCmd represents the cluster up command
var clusterDownCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroys a local docker-based kubernetes cluster",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return helpers.WithCLIContext(context.Background(), destroy)
	},
}

//nolint: gocyclo
func create(ctx context.Context) (err error) {
	if masters < 1 {
		return fmt.Errorf("number of masters can't be less than 1")
	}

	nanoCPUs, err := parseCPUShare()
	if err != nil {
		return fmt.Errorf("error parsing --cpus: %s", err)
	}

	memory := int64(clusterMemory) * 1024 * 1024

	// Validate CIDR range and allocate IPs
	fmt.Println("validating CIDR and reserving master IPs")

	fmt.Println("generating PKI and tokens")

	ips := make([]string, masters)
	for i := range ips {
		ips[i] = fmt.Sprintf(baseNetwork, i+2)
	}

	provisioner, err := docker.NewProvisioner(ctx)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint: errcheck

	request := provision.ClusterRequest{
		Name: clusterName,

		Network: provision.NetworkRequest{
			Name: clusterName,
			CIDR: *cidr,
			MTU:  networkMTU,
		},

		Image:             nodeImage,
		KubernetesVersion: kubernetesVersion,
	}

	// Create the master nodes.
	for i := 0; i < masters; i++ {
		var typ generate.Type
		if i == 0 {
			typ = generate.TypeInit
		} else {
			typ = generate.TypeControlPlane
		}

		request.Nodes = append(request.Nodes,
			provision.NodeRequest{
				Type:     typ,
				Name:     fmt.Sprintf("%s-master-%d", clusterName, i+1),
				IP:       net.ParseIP(ips[i]),
				Memory:   memory,
				NanoCPUs: nanoCPUs,
			})
	}

	for i := 1; i <= workers; i++ {
		request.Nodes = append(request.Nodes,
			provision.NodeRequest{
				Type:     generate.TypeJoin,
				Name:     fmt.Sprintf("%s-worker-%d", clusterName, i),
				Memory:   memory,
				NanoCPUs: nanoCPUs,
			})
	}

	cluster, err := provisioner.Create(ctx, request)
	if err != nil {
		return err
	}

	// Create and save the osctl configuration file.
	if err = saveConfig(cluster); err != nil {
		return err
	}

	if !clusterWait {
		return nil
	}

	// Run cluster readiness checks
	checkCtx, checkCtxCancel := context.WithTimeout(ctx, 20*time.Minute)
	defer checkCtxCancel()

	clusterAccess := access.NewAdapter(cluster)
	defer clusterAccess.Close() //nolint: errcheck

	return check.Wait(checkCtx, clusterAccess, check.DefaultClusterChecks(), check.StderrReporter())
}

func destroy(ctx context.Context) error {
	provisioner, err := docker.NewProvisioner(ctx)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint: errcheck

	cluster, err := provisioner.(provision.ClusterNameReflector).Reflect(ctx, clusterName)
	if err != nil {
		return err
	}

	return provisioner.Destroy(ctx, cluster)
}

func saveConfig(cluster provision.Cluster) (err error) {
	newConfig := cluster.TalosConfig()

	c, err := config.Open(talosconfig)
	if err != nil {
		return err
	}

	if c.Contexts == nil {
		c.Contexts = map[string]*config.Context{}
	}

	c.Contexts[cluster.Info().ClusterName] = newConfig.Contexts[cluster.Info().ClusterName]

	c.Context = cluster.Info().ClusterName

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
	clusterUpCmd.Flags().IntVar(&networkMTU, "mtu", 1500, "MTU of the docker bridge network")
	clusterUpCmd.Flags().StringVar(&networkCIDR, "cidr", "10.5.0.0/24", "CIDR of the docker bridge network")
	clusterUpCmd.Flags().IntVar(&workers, "workers", 1, "the number of workers to create")
	clusterUpCmd.Flags().IntVar(&masters, "masters", 1, "the number of masters to create")
	clusterUpCmd.Flags().StringVar(&clusterCpus, "cpus", "1.5", "the share of CPUs as fraction (each container)")
	clusterUpCmd.Flags().IntVar(&clusterMemory, "memory", 1024, "the limit on memory usage in MB (each container)")
	clusterUpCmd.Flags().BoolVar(&clusterWait, "wait", false, "wait for the cluster to be ready before returning")
	clusterUpCmd.Flags().StringVar(&kubernetesVersion, "kubernetes-version", constants.DefaultKubernetesVersion, "desired kubernetes version to run")
	clusterCmd.PersistentFlags().StringVar(&clusterName, "name", "talos_default", "the name of the cluster")
	clusterCmd.AddCommand(clusterUpCmd)
	clusterCmd.AddCommand(clusterDownCmd)
	rootCmd.AddCommand(clusterCmd)
}
