// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"

	"github.com/docker/cli/opts"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/images"
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
		common: getDefaultCommonOptions(),
		docker: dockerOps{
			hostIP:     "0.0.0.0",
			talosImage: helpers.DefaultImage(images.DefaultTalosImageRepository),
		},
	}

	const (
		portsFlag             = "exposed-ports"
		dockerDisableIPv6Flag = "disable-ipv6"
		dockerHostIPFlag      = "host-ip"
		mountOptsFlag         = "mount"
		subnetFlag            = "subnet"
	)

	getDockerFlags := func() *pflag.FlagSet {
		docker := pflag.NewFlagSet("docker", pflag.PanicOnError)

		docker.StringVarP(&ops.docker.ports, portsFlag, "p", ops.docker.ports,
			"comma-separated list of ports/protocols to expose on init node. Ex -p <hostPort>:<containerPort>/<protocol (tcp or udp)>")
		docker.StringVar(&ops.docker.hostIP, dockerHostIPFlag, ops.docker.hostIP, "Host IP to forward exposed ports to")
		docker.BoolVar(&ops.docker.disableIPv6, dockerDisableIPv6Flag, ops.docker.disableIPv6, "skip enabling IPv6 in containers")
		cli.Should(docker.MarkHidden(dockerDisableIPv6Flag))
		docker.Var(&ops.docker.mountOpts, mountOptsFlag, "attach a mount to the container (docker --mount syntax)")
		docker.StringVar(&ops.docker.talosImage, "image", ops.docker.talosImage, "the talos image to run")

		return docker
	}

	commonFlags := getCommonUserFacingFlags(&ops.common)
	commonFlags.StringVar(&ops.common.networkCIDR, subnetFlag, ops.common.networkCIDR, "Docker network subnet CIDR")

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

				err = postCreate(ctx, ops.common, data.talosconfig, cluster, data.provisionOptions, data.clusterRequest)
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
