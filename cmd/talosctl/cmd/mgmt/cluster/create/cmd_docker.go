// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"time"

	"github.com/docker/cli/opts"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

type dockerOps struct {
	hostIP      string
	disableIPv6 bool
	mountOpts   opts.MountOpt
	ports       string
	talosImage  string
}

func init() {
	ops := &createOps{
		common: commonOps{
			controlplanes:      1,
			networkMTU:         1500,
			clusterWaitTimeout: 20 * time.Minute,
			clusterWait:        true,
			dnsDomain:          "cluster.local",
			controlPlanePort:   constants.DefaultControlPlanePort,
			rootOps:            &clustercmd.Flags,
			networkIPv4:        true,
		},
		docker: dockerOps{},
	}

	const (
		controlPlaneCpusFlag   = "cpus-controlplanes"
		controlPlaneMemoryFlag = "memory-controlplanes"
		workersCpusFlag        = "cpus-workers"
		workersMemoryFlag      = "memory-workers"

		configPatchFlag             = "config-patch"
		configPatchControlPlaneFlag = "config-patch-controlplanes"
		configPatchWorkerFlag       = "config-patch-workers"

		// docker specific flags
		portsFlag             = "exposed-ports"
		dockerDisableIPv6Flag = "disable-ipv6"
		dockerHostIPFlag      = "host-ip"
		mountOptsFlag         = "mount"
		subnetFlag            = "subnet"
	)

	getDockerFlags := func() *pflag.FlagSet {
		docker := pflag.NewFlagSet("docker", pflag.PanicOnError)

		docker.StringVarP(&ops.docker.ports, portsFlag, "p", "",
			"comma-separated list of ports/protocols to expose on init node. Ex -p <hostPort>:<containerPort>/<protocol (tcp or udp)>")
		docker.StringVar(&ops.docker.hostIP, dockerHostIPFlag, "0.0.0.0", "Host IP to forward exposed ports to")
		docker.BoolVar(&ops.docker.disableIPv6, dockerDisableIPv6Flag, false, "skip enabling IPv6 in containers")
		cli.Should(docker.MarkHidden(dockerDisableIPv6Flag))
		docker.Var(&ops.docker.mountOpts, mountOptsFlag, "attach a mount to the container (docker --mount syntax)")
		docker.StringVar(&ops.docker.talosImage, "image", helpers.DefaultImage(images.DefaultTalosImageRepository), "the talos image to run")

		return docker
	}

	getCommonFlags := func() *pflag.FlagSet {
		common := pflag.NewFlagSet("common", pflag.PanicOnError)

		common.StringVar(&ops.common.networkCIDR, subnetFlag, "10.5.0.0/24", "Docker network subnet CIDR")

		addWorkersFlag(common, &ops.common.workers)
		addKubernetesVersionFlag(common, &ops.common.kubernetesVersion)
		addTalosconfigDestinationFlag(common, &ops.common.talosconfigDestination, talosconfigDestinationFlagName)
		addConfigPatchFlag(common, &ops.common.configPatch, configPatchFlag)
		addConfigPatchControlPlaneFlag(common, &ops.common.configPatchControlPlane, configPatchControlPlaneFlag)
		addConfigPatchWorkerFlag(common, &ops.common.configPatchWorker, configPatchWorkerFlag)

		// the following flags are used in tests
		addNetworkMTUFlag(common, &ops.common.networkMTU)
		cli.Should(common.MarkHidden(networkMTUFlagName))
		addRegistryMirrorFlag(common, &ops.common.registryMirrors)
		cli.Should(common.MarkHidden(registryMirrorFlagName))

		addControlplaneCpusFlag(common, &ops.common.controlplaneResources.cpu, controlPlaneCpusFlag)
		addWorkersCpusFlag(common, &ops.common.workerResources.cpu, workersCpusFlag)
		addControlPlaneMemoryFlag(common, &ops.common.controlplaneResources.memory, controlPlaneMemoryFlag)
		addWorkersMemoryFlag(common, &ops.common.workerResources.memory, workersMemoryFlag)

		return common
	}

	createDockerCmd := &cobra.Command{
		Use:   "docker",
		Short: "Create a local Docker based kubernetes cluster",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.WithContext(context.Background(), func(ctx context.Context) error {
				provisioner, err := providers.Factory(ctx, providers.DockerProviderName)
				if err != nil {
					return err
				}

				data, err := getDockerClusterRequest(ops.common, ops.docker, provisioner)
				if err != nil {
					return err
				}

				cluster, err := provisioner.Create(ctx, data.clusterRequest, data.provisionOptions...)
				if err != nil {
					return err
				}
				// Create and save the talosctl configuration file.
				err = postCreate(ctx, ops.common, data.talosconfig, cluster, data.provisionOptions, data.clusterRequest)
				if err != nil {
					return err
				}

				return clustercmd.ShowCluster(cluster)
			})
		},
	}

	createDockerCmd.Flags().AddFlagSet(getDockerFlags())
	createDockerCmd.Flags().AddFlagSet(getCommonFlags())

	createCmd.AddCommand(createDockerCmd)
}
