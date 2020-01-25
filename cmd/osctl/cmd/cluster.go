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
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	clientconfig "github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/internal/pkg/provision/access"
	"github.com/talos-systems/talos/internal/pkg/provision/check"
	"github.com/talos-systems/talos/internal/pkg/provision/providers"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/constants"
	talosnet "github.com/talos-systems/talos/pkg/net"
)

var (
	provisioner             string
	clusterName             string
	nodeImage               string
	nodeInstallImage        string
	nodeVmlinuxPath         string
	nodeInitramfsPath       string
	networkCIDR             string
	networkMTU              int
	nameservers             []string
	workers                 int
	masters                 int
	clusterCpus             string
	clusterMemory           int
	clusterDiskSize         int
	clusterWait             bool
	clusterWaitTimeout      time.Duration
	forceInitNodeAsEndpoint bool
	forceEndpoint           string
	inputDir                string
	cniBinPath              []string
	cniConfDir              string
	cniCacheDir             string
	stateDir                string
)

// clusterCmd represents the cluster command
var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "A collection of commands for managing local docker-based or firecracker-based clusters",
	Long:  ``,
}

// clusterUpCmd represents the cluster up command
var clusterUpCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates a local docker-based or firecracker-based kubernetes cluster",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return helpers.WithCLIContext(context.Background(), create)
	},
}

// clusterDownCmd represents the cluster up command
var clusterDownCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroys a local docker-based or firecracker-based kubernetes cluster",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return helpers.WithCLIContext(context.Background(), destroy)
	},
}

// clusterShowCmd represents the cluster show command
var clusterShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Shows info about a local provisioned kubernetes cluster",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return helpers.WithCLIContext(context.Background(), show)
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
	diskSize := int64(clusterDiskSize) * 1024 * 1024

	// Validate CIDR range and allocate IPs
	fmt.Println("validating CIDR and reserving IPs")

	_, cidr, err := net.ParseCIDR(networkCIDR)
	if err != nil {
		return fmt.Errorf("error validating cidr block: %w", err)
	}

	// Gateway addr at 1st IP in range, ex. 192.168.0.1
	var gatewayIP net.IP

	gatewayIP, err = talosnet.NthIPInNetwork(cidr, 1)
	if err != nil {
		return err
	}

	// Set starting ip at 2nd ip in range, ex: 192.168.0.2
	ips := make([]net.IP, masters+workers)

	for i := range ips {
		ips[i], err = talosnet.NthIPInNetwork(cidr, i+2)
		if err != nil {
			return err
		}
	}

	// Parse nameservers
	nameserverIPs := make([]net.IP, len(nameservers))

	for i := range nameserverIPs {
		nameserverIPs[i] = net.ParseIP(nameservers[i])
		if nameserverIPs[i] == nil {
			return fmt.Errorf("failed parsing nameserver IP %q", nameservers[i])
		}
	}

	provisioner, err := providers.Factory(ctx, provisioner)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint: errcheck

	// Craft cluster and node requests
	request := provision.ClusterRequest{
		Name: clusterName,

		Network: provision.NetworkRequest{
			Name:        clusterName,
			CIDR:        *cidr,
			GatewayAddr: gatewayIP,
			MTU:         networkMTU,
			Nameservers: nameserverIPs,
			CNI: provision.CNIConfig{
				BinPath:  cniBinPath,
				ConfDir:  cniConfDir,
				CacheDir: cniCacheDir,
			},
		},

		Image:             nodeImage,
		KernelPath:        nodeVmlinuxPath,
		InitramfsPath:     nodeInitramfsPath,
		KubernetesVersion: kubernetesVersion,

		SelfExecutable: os.Args[0],
		StateDirectory: stateDir,
	}

	provisionOptions := []provision.Option{}
	configBundleOpts := []config.BundleOption{}

	if inputDir != "" {
		configBundleOpts = append(configBundleOpts, config.WithExistingConfigs(inputDir))
	} else {
		genOptions := []generate.GenOption{
			generate.WithInstallImage(nodeInstallImage),
		}

		genOptions = append(genOptions, provisioner.GenOptions(request.Network)...)

		endpointList := []string{}

		if forceEndpoint != "" {
			endpointList = append(endpointList, forceEndpoint)
			provisionOptions = append(provisionOptions, provision.WithEndpoint(forceEndpoint))
		} else if forceInitNodeAsEndpoint {
			endpointList = append(endpointList, ips[0].String())
		}

		// NB: the localhost endpoint must come last since we currently expect the first endpoint
		// listed to be the default osctl endpoint and that broke CI
		endpointList = append(endpointList, "127.0.0.1")

		genOptions = append(genOptions, generate.WithEndpointList(endpointList))

		configBundleOpts = append(configBundleOpts,
			config.WithInputOptions(
				&config.InputOptions{
					ClusterName: clusterName,
					Endpoint:    fmt.Sprintf("https://%s:6443", ips[0]),
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

	// Create the master nodes.
	for i := 0; i < masters; i++ {
		var cfg runtime.Configurator

		if i == 0 {
			cfg = configBundle.Init()
		} else {
			cfg = configBundle.ControlPlane()
		}

		request.Nodes = append(request.Nodes,
			provision.NodeRequest{
				Name:     fmt.Sprintf("%s-master-%d", clusterName, i+1),
				IP:       ips[i],
				Memory:   memory,
				NanoCPUs: nanoCPUs,
				DiskSize: diskSize,
				Config:   cfg,
			})
	}

	for i := 1; i <= workers; i++ {
		request.Nodes = append(request.Nodes,
			provision.NodeRequest{
				Name:     fmt.Sprintf("%s-worker-%d", clusterName, i),
				IP:       ips[masters+i-1],
				Memory:   memory,
				NanoCPUs: nanoCPUs,
				DiskSize: diskSize,
				Config:   configBundle.Join(),
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
	provisioner, err := providers.Factory(ctx, provisioner)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint: errcheck

	cluster, err := provisioner.Reflect(ctx, clusterName, stateDir)
	if err != nil {
		return err
	}

	return provisioner.Destroy(ctx, cluster)
}

func show(ctx context.Context) error {
	provisioner, err := providers.Factory(ctx, provisioner)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint: errcheck

	cluster, err := provisioner.Reflect(ctx, clusterName, stateDir)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "PROVISIONER\t%s\n", cluster.Provisioner())
	fmt.Fprintf(w, "NAME\t%s\n", cluster.Info().ClusterName)
	fmt.Fprintf(w, "NETWORK NAME\t%s\n", cluster.Info().Network.Name)

	ones, _ := cluster.Info().Network.CIDR.Mask.Size()
	fmt.Fprintf(w, "NETWORK CIDR\t%s/%d\n", cluster.Info().Network.CIDR.IP, ones)
	fmt.Fprintf(w, "NETWORK GATEWAY\t%s\n", cluster.Info().Network.GatewayAddr)
	fmt.Fprintf(w, "NETWORK MTU\t%d\n", cluster.Info().Network.MTU)

	if err = w.Flush(); err != nil {
		return err
	}

	fmt.Fprint(os.Stdout, "\nNODES:\n\n")

	w = tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	fmt.Fprintf(w, "NAME\tTYPE\tIP\tCPU\tRAM\tDISK\n")

	nodes := cluster.Info().Nodes
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })

	for _, node := range nodes {
		cpus := "-"
		if node.NanoCPUs > 0 {
			cpus = fmt.Sprintf("%.2f", float64(node.NanoCPUs)/1000.0/1000.0/1000.0)
		}

		mem := "-"
		if node.Memory > 0 {
			mem = humanize.Bytes(uint64(node.Memory))
		}

		disk := "-"
		if node.DiskSize > 0 {
			disk = humanize.Bytes(uint64(node.DiskSize))
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			node.Name,
			node.Type,
			node.PrivateIP,
			cpus,
			mem,
			disk,
		)
	}

	return w.Flush()
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
	defaultStateDir, err := clientconfig.GetTalosDirectory()
	if err == nil {
		defaultStateDir = filepath.Join(defaultStateDir, "clusters")
	}

	clusterUpCmd.Flags().StringVar(&nodeImage, "image", defaultImage(constants.DefaultTalosImageRepository), "the image to use")
	clusterUpCmd.Flags().StringVar(&nodeInstallImage, "install-image", defaultImage(constants.DefaultInstallerImageRepository), "the installer image to use")
	clusterUpCmd.Flags().StringVar(&nodeVmlinuxPath, "vmlinux-path", helpers.ArtifactPath(constants.KernelUncompressedAsset), "the uncompressed kernel image to use")
	clusterUpCmd.Flags().StringVar(&nodeInitramfsPath, "initrd-path", helpers.ArtifactPath(constants.InitramfsAsset), "the uncompressed kernel image to use")
	clusterUpCmd.Flags().IntVar(&networkMTU, "mtu", 1500, "MTU of the docker bridge network")
	clusterUpCmd.Flags().StringVar(&networkCIDR, "cidr", "10.5.0.0/24", "CIDR of the docker bridge network")
	clusterUpCmd.Flags().StringSliceVar(&nameservers, "nameservers", []string{"8.8.8.8", "1.1.1.1"}, "list of nameservers to use (VM only)")
	clusterUpCmd.Flags().IntVar(&workers, "workers", 1, "the number of workers to create")
	clusterUpCmd.Flags().IntVar(&masters, "masters", 1, "the number of masters to create")
	clusterUpCmd.Flags().StringVar(&clusterCpus, "cpus", "1.5", "the share of CPUs as fraction (each container)")
	clusterUpCmd.Flags().IntVar(&clusterMemory, "memory", 1024, "the limit on memory usage in MB (each container)")
	clusterUpCmd.Flags().IntVar(&clusterDiskSize, "disk", 4*1024, "the limit on disk size in MB (each VM)")
	clusterUpCmd.Flags().BoolVar(&clusterWait, "wait", false, "wait for the cluster to be ready before returning")
	clusterUpCmd.Flags().DurationVar(&clusterWaitTimeout, "wait-timeout", 20*time.Minute, "timeout to wait for the cluster to be ready")
	clusterUpCmd.Flags().BoolVar(&forceInitNodeAsEndpoint, "init-node-as-endpoint", false, "use init node as endpoint instead of any load balancer endpoint")
	clusterUpCmd.Flags().StringVar(&forceEndpoint, "endpoint", "", "use endpoint instead of provider defaults")
	clusterUpCmd.Flags().StringVar(&kubernetesVersion, "kubernetes-version", constants.DefaultKubernetesVersion, "desired kubernetes version to run")
	clusterUpCmd.Flags().StringVarP(&inputDir, "input-dir", "i", "", "location of pre-generated config files")
	clusterUpCmd.Flags().StringSliceVar(&cniBinPath, "cni-bin-path", []string{"/opt/cni/bin"}, "search path for CNI binaries")
	clusterUpCmd.Flags().StringVar(&cniConfDir, "cni-conf-dir", "/etc/cni/conf.d", "CNI config directory path")
	clusterUpCmd.Flags().StringVar(&cniCacheDir, "cni-cache-dir", "/var/lib/cni", "CNI cache directory path")
	clusterCmd.PersistentFlags().StringVar(&provisioner, "provisioner", "docker", "Talos cluster provisioner to use")
	clusterCmd.PersistentFlags().StringVar(&stateDir, "state", defaultStateDir, "directory path to store cluster state")
	clusterCmd.PersistentFlags().StringVar(&clusterName, "name", "talos-default", "the name of the cluster")
	clusterCmd.AddCommand(clusterUpCmd)
	clusterCmd.AddCommand(clusterDownCmd)
	clusterCmd.AddCommand(clusterShowCmd)
	rootCmd.AddCommand(clusterCmd)
}
