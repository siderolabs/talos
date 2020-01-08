// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/spf13/cobra"

	clientconfig "github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/internal/pkg/provision/access"
	"github.com/talos-systems/talos/internal/pkg/provision/check"
	"github.com/talos-systems/talos/internal/pkg/provision/providers/docker"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/constants"
	talosnet "github.com/talos-systems/talos/pkg/net"
)

var (
	clusterName             string
	nodeImage               string
	networkCIDR             string
	networkMTU              int
	workers                 int
	masters                 int
	clusterCpus             string
	clusterMemory           int
	clusterWait             bool
	clusterWaitTimeout      time.Duration
	forceInitNodeAsEndpoint bool
	forceEndpoint           string
	inputDir                string
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

	provisioner, err := docker.NewProvisioner(ctx)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint: errcheck

	provisionOptions := []provision.Option{}
	configBundleOpts := []config.BundleOption{}

	if inputDir != "" {
		configBundleOpts = append(configBundleOpts, config.WithExistingConfigs(inputDir))
	} else {
		var genOptions []generate.GenOption

		if forceEndpoint != "" {
			genOptions = append(genOptions, generate.WithEndpointList([]string{forceEndpoint}))
			provisionOptions = append(provisionOptions, provision.WithEndpoint(forceEndpoint))
		} else if forceInitNodeAsEndpoint {
			genOptions = append(genOptions, generate.WithEndpointList([]string{ips[0]}))
		}

		configBundleOpts = append(configBundleOpts,
			config.WithInputOptions(
				&config.InputOptions{
					ClusterName: clusterName,
					MasterIPs:   ips,
					KubeVersion: kubernetesVersion,
					GenOptions:  genOptions,
				}),
		)
	}

	configBundle, err := config.NewConfigBundle(configBundleOpts...)
	if err != nil {
		return err
	}

	// Add talosconfig to provision options so we'll have it to parse there
	provisionOptions = append(provisionOptions, provision.WithTalosConfig(configBundle.TalosConfig()))

	// Craft cluster and node requests
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

		var configDataStruct *v1alpha1.Config

		var configDataString string

		if i == 0 {
			typ = generate.TypeInit
			configDataStruct = configBundle.InitCfg
		} else {
			typ = generate.TypeControlPlane
			configDataStruct = configBundle.ControlPlaneCfg
		}

		configDataString, err = configDataStruct.String()
		if err != nil {
			return err
		}

		request.Nodes = append(request.Nodes,
			provision.NodeRequest{
				Type:       typ,
				Name:       fmt.Sprintf("%s-master-%d", clusterName, i+1),
				IP:         net.ParseIP(ips[i]),
				Memory:     memory,
				NanoCPUs:   nanoCPUs,
				ConfigData: base64.StdEncoding.EncodeToString([]byte(configDataString)),
			})
	}

	for i := 1; i <= workers; i++ {
		var configDataString string

		configDataString, err = configBundle.Join().String()
		if err != nil {
			return err
		}

		request.Nodes = append(request.Nodes,
			provision.NodeRequest{
				Type:       generate.TypeJoin,
				Name:       fmt.Sprintf("%s-worker-%d", clusterName, i),
				Memory:     memory,
				NanoCPUs:   nanoCPUs,
				ConfigData: base64.StdEncoding.EncodeToString([]byte(configDataString)),
			})
	}

	cluster, err := provisioner.Create(ctx, request, provisionOptions...)
	if err != nil {
		return err
	}

	// Create and save the osctl configuration file.
	if err = saveConfig(cluster, configBundle.TalosConfig()); err != nil {
		return err
	}

	if !clusterWait {
		return nil
	}

	// Run cluster readiness checks
	checkCtx, checkCtxCancel := context.WithTimeout(ctx, clusterWaitTimeout)
	defer checkCtxCancel()

	clusterAccess := access.NewAdapter(cluster, provisionOptions...)
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

func saveConfig(cluster provision.Cluster, talosConfigObj *clientconfig.Config) (err error) {
	c, err := clientconfig.Open(talosconfig)
	if err != nil {
		return err
	}

	if c.Contexts == nil {
		c.Contexts = map[string]*clientconfig.Context{}
	}

	c.Contexts[cluster.Info().ClusterName] = talosConfigObj.Contexts[cluster.Info().ClusterName]

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
	clusterUpCmd.Flags().DurationVar(&clusterWaitTimeout, "wait-timeout", 20*time.Minute, "timeout to wait for the cluster to be ready")
	clusterUpCmd.Flags().BoolVar(&forceInitNodeAsEndpoint, "init-node-as-endpoint", false, "use init node as endpoint instead of any load balancer endpoint")
	clusterUpCmd.Flags().StringVar(&forceEndpoint, "endpoint", "", "use endpoint instead of provider defaults")
	clusterUpCmd.Flags().StringVar(&kubernetesVersion, "kubernetes-version", constants.DefaultKubernetesVersion, "desired kubernetes version to run")
	clusterUpCmd.Flags().StringVarP(&inputDir, "input-dir", "i", "", "location of pre-generated config files")
	clusterCmd.PersistentFlags().StringVar(&clusterName, "name", "talos-default", "the name of the cluster")
	clusterCmd.AddCommand(clusterUpCmd)
	clusterCmd.AddCommand(clusterDownCmd)
	rootCmd.AddCommand(clusterCmd)
}
