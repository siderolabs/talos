// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/blang/semver/v4"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	mgmthelpers "github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/artifacts"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services/registry"
	"github.com/siderolabs/talos/pkg/imager/cache"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
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
	Short:   "Manage CRI container images",
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
	Use:     "pull <image>",
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

		if slices.Contains([]string{provisionerInstaller, provisionerAll}, imageDefaultCmdFlags.provisioner.String()) {
			fmt.Printf("%s\n", images.Installer)
		}

		if slices.Contains([]string{provisionerDocker, provisionerAll}, imageDefaultCmdFlags.provisioner.String()) {
			fmt.Printf("%s\n", images.Talos)
		}

		fmt.Printf("%s\n", images.Pause)

		return nil
	},
}

const (
	provisionerDocker    = "docker"
	provisionerInstaller = "installer"
	provisionerAll       = "all"
)

var imageDefaultCmdFlags = struct {
	provisioner pflag.Value
}{
	provisioner: helpers.StringChoice(provisionerInstaller, provisionerDocker, provisionerAll),
}

// imageSourceBundleCmd represents the image source-bundle command.
var imageSourceBundleCmd = &cobra.Command{
	Use:   "source-bundle <talos-version>",
	Short: "List the source images used for building Talos",
	Long:  ``,
	Args: cobra.MatchAll(
		cobra.ExactArgs(1),
		func(cmd *cobra.Command, args []string) error {
			maximumVersion, err := semver.ParseTolerant(version.Tag)
			if err != nil {
				panic(err) // panic, this should never happen
			}

			tag := args[0]

			ver, err := semver.ParseTolerant(tag)
			if err != nil {
				return fmt.Errorf("invalid argument %q for %q: tag must be a valid semver", tag, cmd.CommandPath())
			}

			if !ver.GTE(minimumVersion) || !ver.LT(maximumVersion) {
				return fmt.Errorf("invalid argument %q for %q: tag for the bundle must be within range \"v%s\" - \"v%s\"", tag, cmd.CommandPath(), minimumVersion, maximumVersion)
			}

			return nil
		},
	),
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			tag        = args[0]
			err        error
			extensions []artifacts.ExtensionRef
			overlays   []artifacts.OverlayRef
		)

		sources := images.ListSourcesFor(tag)

		extensions, err = artifacts.FetchOfficialExtensions(tag)
		if err != nil {
			return fmt.Errorf("error fetching official extensions for %s: %w", tag, err)
		}

		overlays, err = artifacts.FetchOfficialOverlays(tag)
		if err != nil {
			return fmt.Errorf("error fetching official overlays for %s: %w", tag, err)
		}

		fmt.Printf("%s\n", sources.Installer)
		fmt.Printf("%s\n", sources.InstallerBase)
		fmt.Printf("%s\n", sources.Imager)
		fmt.Printf("%s\n", sources.Talos)
		fmt.Printf("%s\n", sources.TalosctlAll)
		fmt.Printf("%s\n", sources.Overlays)
		fmt.Printf("%s\n", sources.Extensions)

		digestedReferences := []string{}

		for _, overlay := range overlays {
			digestedReferences = append(digestedReferences, fmt.Sprintf("%s@%s", overlay.TaggedReference.String(), overlay.Digest))
		}

		for _, extension := range extensions {
			digestedReferences = append(digestedReferences, fmt.Sprintf("%s@%s", extension.TaggedReference.String(), extension.Digest))
		}

		slices.Sort(digestedReferences)

		for _, ref := range slices.Compact(digestedReferences) {
			fmt.Printf("%s\n", ref)
		}

		return nil
	},
}

var minimumVersion = semver.MustParse("1.11.0-alpha.0")

// imageIntegrationCmd represents the integration image command.
var imageIntegrationCmd = &cobra.Command{
	Use:    "integration",
	Short:  "List the integration images used by k8s in Talos",
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(*cobra.Command, []string) error {
		if stat, _ := os.Stdin.Stat(); (stat.Mode() & os.ModeCharDevice) != 0 { //nolint:errcheck
			return errors.New("input must be piped")
		}

		if imageIntegrationCmdFlags.installerTag == "" {
			return errors.New("installer tag is required")
		}

		if imageIntegrationCmdFlags.registryAndUser == "" {
			return errors.New("registry and user string is required")
		}

		imgs := images.List(container.NewV1Alpha1(&v1alpha1.Config{
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

		imageNames := []string{
			imgs.Flannel,
			imgs.CoreDNS,
			imgs.Etcd,
			imgs.KubeAPIServer,
			imgs.KubeControllerManager,
			imgs.KubeScheduler,
			imgs.KubeProxy,
			imgs.Kubelet,
			imgs.Pause,
			"registry.k8s.io/conformance:v" + constants.DefaultKubernetesVersion,
			"docker.io/library/alpine:latest",
			"ghcr.io/siderolabs/talosctl:latest",
			"registry.k8s.io/kube-apiserver:v1.27.0",
			imageIntegrationCmdFlags.registryAndUser + "/installer:" +
				imageIntegrationCmdFlags.installerTag,
			imageIntegrationCmdFlags.registryAndUser + "/talos:" +
				imageIntegrationCmdFlags.talosTag,
		}

		sc := bufio.NewScanner(os.Stdin)

		for sc.Scan() {
			switch sc := sc.Text(); {
			case strings.Contains(sc, "authenticated-"):
			// skip authenticated images
			case strings.HasPrefix(sc, "invalid.registry.k8s.io"):
			// skip invalid images
			default:
				imageNames = append(imageNames, sc)
			}
		}

		if err := sc.Err(); err != nil {
			return fmt.Errorf("error reading from stdin: %w", err)
		}

		slices.Sort(imageNames)

		imageNames = slices.Compact(imageNames)

		for _, img := range imageNames {
			fmt.Println(img)
		}

		return nil
	},
}

var imageIntegrationCmdFlags struct {
	installerTag    string
	talosTag        string
	registryAndUser string
}

// imageCacheCreate represents the image cache create command.
var imageCacheCreateCmd = &cobra.Command{
	Use:   "cache-create",
	Short: "Create a cache of images in OCI format into a directory",
	Long:  `Create a cache of images in OCI format into a directory`,
	Example: fmt.Sprintf(
		`talosctl images cache-create --images=ghcr.io/siderolabs/kubelet:v%s --image-cache-path=/tmp/talos-image-cache

Alternatively, stdin can be piped to the command:
talosctl images default | talosctl images cache-create --image-cache-path=/tmp/talos-image-cache --images=-
`,
		constants.DefaultKubernetesVersion,
	),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(imageCacheCreateCmdFlags.images) == 0 {
			return fmt.Errorf("no images specified")
		}

		if imageCacheCreateCmdFlags.force {
			if err := os.RemoveAll(imageCacheCreateCmdFlags.imageCachePath); err != nil {
				return fmt.Errorf("error removing existing image cache path %s: %w", imageCacheCreateCmdFlags.imageCachePath, err)
			}
		}

		if _, err := os.Stat(imageCacheCreateCmdFlags.imageCachePath); err == nil {
			return fmt.Errorf("image cache path %s already exists, use --force to remove and use the path", imageCacheCreateCmdFlags.imageCachePath)
		}

		if imageCacheCreateCmdFlags.images[0] == "-" {
			var imagesListData strings.Builder

			if _, err := io.Copy(&imagesListData, os.Stdin); err != nil {
				return fmt.Errorf("error reading from stdin: %w", err)
			}

			imageCacheCreateCmdFlags.images = strings.Split(strings.Trim(imagesListData.String(), "\n"), "\n")
		}

		err := cache.Generate(
			imageCacheCreateCmdFlags.images,
			imageCacheCreateCmdFlags.platform,
			imageCacheCreateCmdFlags.insecure,
			imageCacheCreateCmdFlags.imageLayerCachePath,
			imageCacheCreateCmdFlags.imageCachePath,
			imageCacheCreateCmdFlags.layout.String() == layoutFlat,
		)
		if err != nil {
			return fmt.Errorf("error generating cache: %w", err)
		}

		return nil
	},
}

const (
	layoutOCI  = "oci"
	layoutFlat = "flat"
)

var imageCacheCreateCmdFlags = struct {
	imageCachePath      string
	imageLayerCachePath string
	layout              pflag.Value
	platform            []string

	images []string

	insecure bool
	force    bool
}{
	layout: helpers.StringChoice(layoutOCI, layoutFlat),
}

// imageCacheServeCmd represents the image cache serve command.
var imageCacheServeCmd = &cobra.Command{
	Use:     "cache-serve",
	Short:   "Serve an OCI image cache directory over HTTP(S) as a container registry",
	Long:    `Serve an OCI image cache directory over HTTP(S) as a container registry`,
	Example: ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer cancel()

		development, err := zap.NewDevelopment()
		if err != nil {
			return fmt.Errorf("failed to create development logger: %w", err)
		}

		it := func(yield func(string) bool) {
			for _, root := range []string{imageCacheServeCmdFlags.imageCachePath} {
				if !yield(root) {
					return
				}
			}
		}

		return registry.NewService(registry.NewMultiPathFS(it), development).Run(
			ctx,
			registry.WithTLS(
				imageCacheServeCmdFlags.tlsCertFile,
				imageCacheServeCmdFlags.tlsKeyFile,
			),
			registry.WithAddress(imageCacheServeCmdFlags.address),
		)
	},
}

var imageCacheServeCmdFlags struct {
	imageCachePath string
	address        string
	tlsCertFile    string
	tlsKeyFile     string
}

// imageCacheCertGenCmd represents the image cache tls certificate generation command.
var imageCacheCertGenCmd = &cobra.Command{
	Use:     "cache-cert-gen",
	Short:   "Generate TLS certificates and CA patch required for securing image cache to Talos communication",
	Long:    `Generate TLS certificates and CA patch required for securing image cache to Talos communication`,
	Example: ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		caPEM, certPEM, keyPEM, err := mgmthelpers.GenerateSelfSignedCert(imageCacheCertGenCmdFlags.advertisedAddress)
		if err != nil {
			return nil
		}

		if err = generateConfigPatch(caPEM); err != nil {
			return err
		}

		if err := os.WriteFile(imageCacheCertGenCmdFlags.tlsCaFile, caPEM, 0o644); err != nil {
			return err
		}

		if err := os.WriteFile(imageCacheCertGenCmdFlags.tlsCertFile, certPEM, 0o644); err != nil {
			return err
		}

		if err := os.WriteFile(imageCacheCertGenCmdFlags.tlsKeyFile, keyPEM, 0o600); err != nil {
			return err
		}

		return nil
	},
}

func generateConfigPatch(caPEM []byte) error {
	patch := security.NewTrustedRootsConfigV1Alpha1()
	patch.MetaName = "image-cache-ca"
	patch.Certificates = string(caPEM)

	ctr, err := container.New(patch)
	if err != nil {
		return err
	}

	patchBytes, err := ctr.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return err
	}

	const patchFile = "image-cache-patch.yaml"

	log.Printf("writing config patch to %s", patchFile)

	return os.WriteFile(patchFile, patchBytes, 0o644)
}

var imageCacheCertGenCmdFlags struct {
	advertisedAddress []net.IP
	tlsCaFile         string
	tlsCertFile       string
	tlsKeyFile        string
}

func init() {
	imageCmd.PersistentFlags().StringVar(&imageCmdFlags.namespace, "namespace", "cri", "namespace to use: `system` (etcd and kubelet images) or `cri` for all Kubernetes workloads")
	addCommand(imageCmd)

	imageCmd.AddCommand(imageDefaultCmd)
	imageDefaultCmd.PersistentFlags().Var(imageDefaultCmdFlags.provisioner, "provisioner", "include provisioner specific images")

	imageCmd.AddCommand(imageListCmd)
	imageCmd.AddCommand(imagePullCmd)
	imageCmd.AddCommand(imageSourceBundleCmd)

	imageCmd.AddCommand(imageCacheCreateCmd)
	imageCacheCreateCmd.PersistentFlags().StringVar(&imageCacheCreateCmdFlags.imageCachePath, "image-cache-path", "", "directory to save the image cache in OCI format")
	imageCacheCreateCmd.MarkPersistentFlagRequired("image-cache-path") //nolint:errcheck
	imageCacheCreateCmd.PersistentFlags().StringVar(&imageCacheCreateCmdFlags.imageLayerCachePath, "image-layer-cache-path", "", "directory to save the image layer cache")
	imageCacheCreateCmd.PersistentFlags().Var(imageCacheCreateCmdFlags.layout, "layout",
		"Specifies the cache layout format: \"oci\" for an OCI image layout directory, or \"flat\" for a registry-like flat file structure")
	imageCacheCreateCmd.PersistentFlags().StringSliceVar(&imageCacheCreateCmdFlags.platform, "platform", []string{"linux/amd64"}, "platform to use for the cache")
	imageCacheCreateCmd.PersistentFlags().StringSliceVar(&imageCacheCreateCmdFlags.images, "images", nil, "images to cache")
	imageCacheCreateCmd.MarkPersistentFlagRequired("images") //nolint:errcheck
	imageCacheCreateCmd.PersistentFlags().BoolVar(&imageCacheCreateCmdFlags.insecure, "insecure", false, "allow insecure registries")
	imageCacheCreateCmd.PersistentFlags().BoolVar(&imageCacheCreateCmdFlags.force, "force", false, "force overwrite of existing image cache")

	imageCmd.AddCommand(imageCacheServeCmd)
	imageCacheServeCmd.PersistentFlags().StringVar(&imageCacheServeCmdFlags.imageCachePath, "image-cache-path", "", "directory to save the image cache in flat format")
	imageCacheServeCmd.MarkPersistentFlagRequired("image-cache-path") //nolint:errcheck
	imageCacheServeCmd.PersistentFlags().StringVar(&imageCacheServeCmdFlags.address, "address", constants.RegistrydListenAddress, "address to serve the registry on")
	imageCacheServeCmd.PersistentFlags().StringVar(&imageCacheServeCmdFlags.tlsCertFile, "tls-cert-file", "", "TLS certificate file to use for serving")
	imageCacheServeCmd.PersistentFlags().StringVar(&imageCacheServeCmdFlags.tlsKeyFile, "tls-key-file", "", "TLS key file to use for serving")

	imageCmd.AddCommand(imageCacheCertGenCmd)
	imageCacheCertGenCmd.PersistentFlags().StringVar(&imageCacheCertGenCmdFlags.tlsCaFile, "tls-ca-file", "ca.crt", "TLS certificate authority file")
	imageCacheCertGenCmd.PersistentFlags().StringVar(&imageCacheCertGenCmdFlags.tlsCertFile, "tls-cert-file", "tls.crt", "TLS certificate file to use for serving")
	imageCacheCertGenCmd.PersistentFlags().StringVar(&imageCacheCertGenCmdFlags.tlsKeyFile, "tls-key-file", "tls.key", "TLS key file to use for serving")
	imageCacheCertGenCmd.PersistentFlags().IPSliceVar(&imageCacheCertGenCmdFlags.advertisedAddress, "advertised-address", []net.IP{}, "The address to advertise to the cluster.")
	imageIntegrationCmd.MarkPersistentFlagRequired("advertised-address") //nolint:errcheck

	imageCmd.AddCommand(imageIntegrationCmd)
	imageIntegrationCmd.PersistentFlags().StringVar(&imageIntegrationCmdFlags.installerTag, "installer-tag", "", "tag of the installer image to use")
	imageIntegrationCmd.MarkPersistentFlagRequired("installer-tag") //nolint:errcheck
	imageIntegrationCmd.PersistentFlags().StringVar(&imageIntegrationCmdFlags.talosTag, "talos-tag", version.Tag, "tag of the installer image to use")
	imageIntegrationCmd.PersistentFlags().StringVar(&imageIntegrationCmdFlags.registryAndUser, "registry-and-user", "", "registry and user to use for the images")
	imageIntegrationCmd.MarkPersistentFlagRequired("registry-and-user") //nolint:errcheck
}
