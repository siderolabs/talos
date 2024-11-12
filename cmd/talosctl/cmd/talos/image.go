// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/imager/cache"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

type imageCmdFlagsType struct {
	namespace string
}

var imageCmdFlags imageCmdFlagsType

func (flags imageCmdFlagsType) apiNamespace() (common.ContainerdNamespace, error) {
	switch flags.namespace {
	case "cri":
		return common.ContainerdNamespace_NS_CRI, nil
	case "system":
		return common.ContainerdNamespace_NS_SYSTEM, nil
	default:
		return 0, fmt.Errorf("unsupported namespace %q", flags.namespace)
	}
}

// imagesCmd represents the image command.
var imageCmd = &cobra.Command{
	Use:     "image",
	Aliases: []string{"images"},
	Short:   "Manage CRI containter images",
	Long:    ``,
	Args:    cobra.NoArgs,
}

// imageListCmd represents the image list command.
var imageListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l", "ls"},
	Short:   "List CRI images",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			ns, err := imageCmdFlags.apiNamespace()
			if err != nil {
				return err
			}

			rcv, err := c.ImageList(ctx, ns)
			if err != nil {
				return fmt.Errorf("error listing images: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NODE\tIMAGE\tDIGEST\tSIZE\tCREATED")

			if err = helpers.ReadGRPCStream(rcv, func(msg *machine.ImageListResponse, node string, multipleNodes bool) error {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					node,
					msg.Name,
					msg.Digest,
					humanize.Bytes(uint64(msg.Size)),
					msg.CreatedAt.AsTime().Format(time.RFC3339),
				)

				return nil
			}); err != nil {
				return err
			}

			return w.Flush()
		})
	},
}

// imagePullCmd represents the image pull command.
var imagePullCmd = &cobra.Command{
	Use:     "pull",
	Aliases: []string{"p"},
	Short:   "Pull an image into CRI",
	Long:    ``,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			ns, err := imageCmdFlags.apiNamespace()
			if err != nil {
				return err
			}

			err = c.ImagePull(ctx, ns, args[0])
			if err != nil {
				return fmt.Errorf("error pulling image: %w", err)
			}

			return nil
		})
	},
}

// imageDefaultCmd represents the image default command.
var imageDefaultCmd = &cobra.Command{
	Use:   "default",
	Short: "List the default images used by Talos",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		images := images.List(container.NewV1Alpha1(&v1alpha1.Config{
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
		}))

		fmt.Printf("%s\n", images.Flannel)
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

// imageCacheCreate represents the image cache create command.
var imageCacheCreateCmd = &cobra.Command{
	Use:   "cache-create",
	Short: "Create a cache of images",
	Long:  `Create a oci tarball of image cache that talos imager can use`,
	Example: fmt.Sprintf(
		`talosctl images cache-create --images=ghcr.io/siderolabs/kubelet:%s --image-cache-path=/tmp/cache.tar

Alternatively, stdin can be piped to the command:
talosctl images default | talosctl images cache-create --image-cache-path=/tmp/cache.tar --images=-
`,
		constants.DefaultKubernetesVersion,
	),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(imageCacheCreateCmdFlags.images) == 0 {
			return fmt.Errorf("no images specified")
		}

		if imageCacheCreateCmdFlags.images[0] == "-" {
			var imagesListData bytes.Buffer

			if _, err := io.Copy(&imagesListData, os.Stdin); err != nil {
				return fmt.Errorf("error reading from stdin: %w", err)
			}

			imageCacheCreateCmdFlags.images = strings.Split(strings.Trim(imagesListData.String(), "\n"), "\n")
		}

		err := cache.Generate(
			imageCacheCreateCmdFlags.images,
			imageCacheCreateCmdFlags.platform,
			imageCacheCreateCmdFlags.insecure,
			imageCacheCreateCmdFlags.imageCachePath,
		)
		if err != nil {
			return fmt.Errorf("error generating cache: %w", err)
		}

		return nil
	},
}

type imageCacheCreateCmdFlagsType struct {
	imageCachePath string
	platform       string

	images []string

	insecure bool
}

var imageCacheCreateCmdFlags imageCacheCreateCmdFlagsType

func init() {
	imageCmd.PersistentFlags().StringVar(&imageCmdFlags.namespace, "namespace", "cri", "namespace to use: `system` (etcd and kubelet images) or `cri` for all Kubernetes workloads")
	addCommand(imageCmd)

	imageCmd.AddCommand(imageDefaultCmd)
	imageCmd.AddCommand(imageListCmd)
	imageCmd.AddCommand(imagePullCmd)
	imageCmd.AddCommand(imageCacheCreateCmd)

	imageCacheCreateCmd.PersistentFlags().StringVar(&imageCacheCreateCmdFlags.imageCachePath, "image-cache-path", "", "path to save the image cache")
	imageCacheCreateCmd.MarkPersistentFlagRequired("image-cache-path") //nolint:errcheck
	imageCacheCreateCmd.PersistentFlags().StringVar(&imageCacheCreateCmdFlags.platform, "platform", "linux/amd64", "platform to use for the cache")
	imageCacheCreateCmd.PersistentFlags().StringSliceVar(&imageCacheCreateCmdFlags.images, "images", nil, "images to cache")
	imageCacheCreateCmd.MarkPersistentFlagRequired("images") //nolint:errcheck
	imageCacheCreateCmd.PersistentFlags().BoolVar(&imageCacheCreateCmdFlags.insecure, "insecure", false, "allow insecure registries")
}
