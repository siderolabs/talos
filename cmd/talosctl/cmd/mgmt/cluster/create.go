// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/cluster/check"
	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/internal/pkg/provision/access"
	"github.com/talos-systems/talos/internal/pkg/provision/providers"
	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/client"
	clientconfig "github.com/talos-systems/talos/pkg/client/config"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/constants"
	talosnet "github.com/talos-systems/talos/pkg/net"
	"github.com/talos-systems/talos/pkg/retry"
)

var (
	talosconfig             string
	nodeImage               string
	nodeInstallImage        string
	registryMirrors         []string
	kubernetesVersion       string
	nodeVmlinuxPath         string
	nodeInitramfsPath       string
	bootloaderEmulation     bool
	configDebug             bool
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
	ports                   string
	withInitNode            bool
)

// createCmd represents the cluster up command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates a local docker-based or firecracker-based kubernetes cluster",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.WithContext(context.Background(), create)
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

	provisioner, err := providers.Factory(ctx, provisionerName)
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

		Image:         nodeImage,
		KernelPath:    nodeVmlinuxPath,
		InitramfsPath: nodeInitramfsPath,

		SelfExecutable: os.Args[0],
		StateDirectory: stateDir,
	}

	provisionOptions := []provision.Option{}
	configBundleOpts := []config.BundleOption{}

	if ports != "" {
		if provisionerName != "docker" {
			return fmt.Errorf("exposed-ports flag only supported with docker provisioner")
		}

		portList := strings.Split(ports, ",")
		provisionOptions = append(provisionOptions, provision.WithDockerPorts(portList))
	}

	if bootloaderEmulation {
		provisionOptions = append(provisionOptions, provision.WithBootladerEmulation())
	}

	if inputDir != "" {
		configBundleOpts = append(configBundleOpts, config.WithExistingConfigs(inputDir))
	} else {
		genOptions := []generate.GenOption{
			generate.WithInstallImage(nodeInstallImage),
			generate.WithDebug(configDebug),
		}

		for _, registryMirror := range registryMirrors {
			components := strings.SplitN(registryMirror, "=", 2)
			if len(components) != 2 {
				return fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
			}

			genOptions = append(genOptions, generate.WithRegistryMirror(components[0], components[1]))
		}

		genOptions = append(genOptions, provisioner.GenOptions(request.Network)...)

		defaultInternalLB, defaultExternalLB := provisioner.GetLoadBalancers(request.Network)

		if defaultInternalLB == "" {
			// provisioner doesn't provide internal LB, so use first master node
			defaultInternalLB = ips[0].String()
		}

		var endpointList []string

		switch {
		case forceEndpoint != "":
			endpointList = []string{forceEndpoint}
			provisionOptions = append(provisionOptions, provision.WithEndpoint(forceEndpoint))
		case forceInitNodeAsEndpoint:
			endpointList = []string{ips[0].String()}
		default:
			endpointList = []string{defaultExternalLB}
		}

		genOptions = append(genOptions, generate.WithEndpointList(endpointList))

		configBundleOpts = append(configBundleOpts,
			config.WithInputOptions(
				&config.InputOptions{
					ClusterName: clusterName,
					Endpoint:    fmt.Sprintf("https://%s:6443", defaultInternalLB),
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

		nodeReq := provision.NodeRequest{
			Name:     fmt.Sprintf("%s-master-%d", clusterName, i+1),
			IP:       ips[i],
			Memory:   memory,
			NanoCPUs: nanoCPUs,
			DiskSize: diskSize,
		}

		if i == 0 {
			nodeReq.Ports = []string{"50000:50000/tcp", "6443:6443/tcp"}
		}

		if withInitNode {
			if i == 0 {
				cfg = configBundle.Init()
			} else {
				cfg = configBundle.ControlPlane()
			}
		} else {
			// Any one of the control plane nodes can be the init node, so we use the
			// init config's content, but change the type to a control plane.
			configBundle.InitCfg.MachineConfig.MachineType = runtime.MachineTypeControlPlane.String()
			cfg = configBundle.Init()
		}

		nodeReq.Config = cfg

		request.Nodes = append(request.Nodes, nodeReq)
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

	// Create and save the talosctl configuration file.
	if err = saveConfig(cluster, configBundle.TalosConfig()); err != nil {
		return err
	}

	clusterAccess := access.NewAdapter(cluster, provisionOptions...)
	defer clusterAccess.Close() //nolint: errcheck

	if !withInitNode {
		cli, err := clusterAccess.Client()
		if err != nil {
			return retry.UnexpectedError(err)
		}

		nodes := clusterAccess.NodesByType(runtime.MachineTypeControlPlane)
		if len(nodes) == 0 {
			return fmt.Errorf("expected at least 1 control plane node, got %d", len(nodes))
		}

		sort.Strings(nodes)

		node := nodes[0]

		nodeCtx := client.WithNodes(ctx, node)

		fmt.Println("waiting for API")

		err = retry.Constant(5*time.Minute, retry.WithUnits(500*time.Millisecond)).Retry(func() error {
			retryCtx, cancel := context.WithTimeout(nodeCtx, 500*time.Millisecond)
			defer cancel()

			if _, err = cli.Version(retryCtx); err != nil {
				return retry.ExpectedError(err)
			}

			return nil
		})

		if err != nil {
			return err
		}

		fmt.Println("bootstrapping cluster")

		bootstrapCtx, cancel := context.WithTimeout(nodeCtx, 30*time.Second)
		defer cancel()

		if err = cli.Bootstrap(bootstrapCtx); err != nil {
			return err
		}
	}

	if !clusterWait {
		return nil
	}

	// Run cluster readiness checks
	checkCtx, checkCtxCancel := context.WithTimeout(ctx, clusterWaitTimeout)
	defer checkCtxCancel()

	return check.Wait(checkCtx, clusterAccess, check.DefaultClusterChecks(), check.StderrReporter())
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
	defaultTalosConfig, err := clientconfig.GetDefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to find default Talos config path: %s", err)
	}

	createCmd.Flags().StringVar(&talosconfig, "talosconfig", defaultTalosConfig, "The path to the Talos configuration file")
	createCmd.Flags().StringVar(&nodeImage, "image", helpers.DefaultImage(constants.DefaultTalosImageRepository), "the image to use")
	createCmd.Flags().StringVar(&nodeInstallImage, "install-image", helpers.DefaultImage(constants.DefaultInstallerImageRepository), "the installer image to use")
	createCmd.Flags().StringVar(&nodeVmlinuxPath, "vmlinux-path", helpers.ArtifactPath(constants.KernelUncompressedAsset), "the uncompressed kernel image to use")
	createCmd.Flags().StringVar(&nodeInitramfsPath, "initrd-path", helpers.ArtifactPath(constants.InitramfsAsset), "the uncompressed kernel image to use")
	createCmd.Flags().BoolVar(&bootloaderEmulation, "with-bootloader-emulation", false, "enable bootloader emulation to load kernel and initramfs from disk image")
	createCmd.Flags().StringSliceVar(&registryMirrors, "registry-mirror", []string{}, "list of registry mirrors to use in format: <registry host>=<mirror URL>")
	createCmd.Flags().BoolVar(&configDebug, "with-debug", false, "enable debug in Talos config to send service logs to the console")
	createCmd.Flags().IntVar(&networkMTU, "mtu", 1500, "MTU of the docker bridge network")
	createCmd.Flags().StringVar(&networkCIDR, "cidr", "10.5.0.0/24", "CIDR of the docker bridge network")
	createCmd.Flags().StringSliceVar(&nameservers, "nameservers", []string{"8.8.8.8", "1.1.1.1"}, "list of nameservers to use")
	createCmd.Flags().IntVar(&workers, "workers", 1, "the number of workers to create")
	createCmd.Flags().IntVar(&masters, "masters", 1, "the number of masters to create")
	createCmd.Flags().StringVar(&clusterCpus, "cpus", "1.5", "the share of CPUs as fraction (each container)")
	createCmd.Flags().IntVar(&clusterMemory, "memory", 1024, "the limit on memory usage in MB (each container)")
	createCmd.Flags().IntVar(&clusterDiskSize, "disk", 4*1024, "the limit on disk size in MB (each VM)")
	createCmd.Flags().BoolVar(&clusterWait, "wait", true, "wait for the cluster to be ready before returning")
	createCmd.Flags().DurationVar(&clusterWaitTimeout, "wait-timeout", 20*time.Minute, "timeout to wait for the cluster to be ready")
	createCmd.Flags().BoolVar(&forceInitNodeAsEndpoint, "init-node-as-endpoint", false, "use init node as endpoint instead of any load balancer endpoint")
	createCmd.Flags().StringVar(&forceEndpoint, "endpoint", "", "use endpoint instead of provider defaults")
	createCmd.Flags().StringVar(&kubernetesVersion, "kubernetes-version", constants.DefaultKubernetesVersion, "desired kubernetes version to run")
	createCmd.Flags().StringVarP(&inputDir, "input-dir", "i", "", "location of pre-generated config files")
	createCmd.Flags().StringSliceVar(&cniBinPath, "cni-bin-path", []string{"/opt/cni/bin"}, "search path for CNI binaries")
	createCmd.Flags().StringVar(&cniConfDir, "cni-conf-dir", "/etc/cni/conf.d", "CNI config directory path")
	createCmd.Flags().StringVar(&cniCacheDir, "cni-cache-dir", "/var/lib/cni", "CNI cache directory path")
	createCmd.Flags().StringVarP(&ports,
		"exposed-ports",
		"p",
		"",
		"Comma-separated list of ports/protocols to expose on init node. Ex -p <hostPort>:<containerPort>/<protocol (tcp or udp)> (Docker provisioner only)",
	)
	createCmd.Flags().BoolVar(&withInitNode, "with-init-node", true, "create the cluster with an init node")
	Cmd.AddCommand(createCmd)
}
