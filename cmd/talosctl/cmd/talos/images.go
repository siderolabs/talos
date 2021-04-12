// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/images"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

// imagesCmd represents the images command.
var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "List the default images used by Talos",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		images := images.List(&v1alpha1.Config{
			MachineConfig: &v1alpha1.MachineConfig{
				MachineKubelet: &v1alpha1.KubeletConfig{},
			},
			ClusterConfig: &v1alpha1.ClusterConfig{
				EtcdConfig:              &v1alpha1.EtcdConfig{},
				APIServerConfig:         &v1alpha1.APIServerConfig{},
				ControllerManagerConfig: &v1alpha1.ControllerManagerConfig{},
				SchedulerConfig:         &v1alpha1.SchedulerConfig{},
				CoreDNSConfig:           &v1alpha1.CoreDNS{},
				ProxyConfig:             &v1alpha1.ProxyConfig{},
			},
		})

		fmt.Printf("%s\n", images.Flannel)
		fmt.Printf("%s\n", images.FlannelCNI)
		fmt.Printf("%s\n", images.CoreDNS)
		fmt.Printf("%s\n", images.Etcd)
		fmt.Printf("%s\n", images.KubeAPIServer)
		fmt.Printf("%s\n", images.KubeControllerManager)
		fmt.Printf("%s\n", images.KubeScheduler)
		fmt.Printf("%s\n", images.KubeProxy)
		fmt.Printf("%s\n", images.Kubelet)
		fmt.Printf("%s\n", images.Installer)
		fmt.Printf("%s\n", images.Pause)

		return nil
	},
}

func init() {
	addCommand(imagesCmd)
}
