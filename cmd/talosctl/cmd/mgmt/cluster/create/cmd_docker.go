// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

func init() {
	dOps := clusterops.GetDocker()
	cOps := clusterops.GetCommon()

	const (
		portsFlag             = "exposed-ports"
		dockerDisableIPv6Flag = "disable-ipv6"
		dockerHostIPFlag      = "host-ip"
		mountOptsFlag         = "mount"
		subnetFlag            = "subnet"
	)

	getDockerFlags := func() *pflag.FlagSet {
		docker := pflag.NewFlagSet("docker", pflag.PanicOnError)

		docker.StringVarP(&dOps.Ports, portsFlag, "p", dOps.Ports,
			"comma-separated list of ports/protocols to expose on init node. Ex -p <hostPort>:<containerPort>/<protocol (tcp or udp)>")
		docker.StringVar(&dOps.HostIP, dockerHostIPFlag, dOps.HostIP, "Host IP to forward exposed ports to")
		docker.BoolVar(&dOps.DisableIPv6, dockerDisableIPv6Flag, dOps.DisableIPv6, "skip enabling IPv6 in containers")
		cli.Should(docker.MarkHidden(dockerDisableIPv6Flag))
		docker.Var(&dOps.MountOpts, mountOptsFlag, "attach a mount to the container (docker --mount syntax)")
		docker.StringVar(&dOps.TalosImage, "image", dOps.TalosImage, "the talos image to run")

		return docker
	}

	commonFlags := getCommonUserFacingFlags(&cOps)
	commonFlags.StringVar(&cOps.NetworkCIDR, subnetFlag, cOps.NetworkCIDR, "Docker network subnet CIDR")

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

				clusterConfigs, err := getDockerClusterRequest(cOps, dOps, provisioner)
				if err != nil {
					return err
				}

				cluster, err := provisioner.Create(ctx, clusterConfigs.ClusterRequest, clusterConfigs.ProvisionOptions...)
				if err != nil {
					return err
				}

				err = postCreate(ctx, cOps, cluster, clusterConfigs)
				if err != nil {
					return err
				}

				return clustercmd.ShowCluster(cluster)
			})
		},
	}

	createDockerCmd.Flags().AddFlagSet(getDockerFlags())
	createDockerCmd.Flags().AddFlagSet(commonFlags)

	createCmd.AddCommand(createDockerCmd)
}
