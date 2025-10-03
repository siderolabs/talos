// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/blang/semver/v4"
	"github.com/dustin/go-humanize"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/olareg/olareg"
	"github.com/olareg/olareg/config"
	"github.com/siderolabs/go-pointer"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/artifacts"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/imager/cache"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
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
			imageIntegrationCmdFlags.registryAndUser + "/installer:" +
				imageIntegrationCmdFlags.installerTag,
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
		)
		if err != nil {
			return fmt.Errorf("error generating cache: %w", err)
		}

		return nil
	},
}

var imageCacheCreateCmdFlags struct {
	imageCachePath      string
	imageLayerCachePath string
	platform            string

	images []string

	insecure bool
	force    bool
}

var imageRegistryCommand = &cobra.Command{
	Use:   "registry",
	Short: "Commands for working with a local image registry",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		logs.Progress.SetOutput(os.Stderr)

		transport := remote.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint: gosec
		}

		options = append(options,
			crane.WithJobs(0),
			crane.Insecure,
			crane.WithNondistributable(),
			crane.WithTransport(transport),
		)
	},
}

var options []crane.Option

var imageRegistryCreateCommand = &cobra.Command{
	Use:   "create <path>",
	Short: "Create a local OCI from a list of images",
	Long:  `Create a local OCI from a list of images`,
	Example: fmt.Sprintf(
		`talosctl images registry create --images=ghcr.io/siderolabs/kubelet:v%s /tmp/registry

Alternatively, stdin can be piped to the command:
talosctl images source-bundle | talosctl images registry create /tmp/registry --images=-
`,
		constants.DefaultKubernetesVersion,
	),
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error

		storagePath := args[0]

		lis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("error finding free port: %w", err)
		}

		addr := lis.Addr().String()

		lis.Close() //nolint:errcheck

		log := slog.New(slog.DiscardHandler)

		if imageRegistryCreateCmdFlags.debug {
			logs.Debug.SetOutput(os.Stderr)
			log = slog.Default()
		}

		reg := olareg.New(config.Config{
			HTTP: config.ConfigHTTP{
				Addr: addr,
			},
			API: config.ConfigAPI{
				PushEnabled:   pointer.To(true),
				DeleteEnabled: pointer.To(false),
				Blob: config.ConfigAPIBlob{
					DeleteEnabled: pointer.To(false),
				},
			},
			Storage: config.ConfigStorage{
				StoreType: config.StoreDir,
				RootDir:   storagePath,
				ReadOnly:  pointer.To(false),
				GC: config.ConfigGC{
					Frequency: -1, // disabled
				},
			},
			Log: log,
		})

		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer cancel()

		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			slog.Info("starting registry", "path", storagePath)

			return reg.Run(ctx)
		})

		eg.Go(func() error {
			<-ctx.Done()

			ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			return reg.Shutdown(ctx2)
		})

		eg.Go(func() error {
			if imageRegistryCreateCmdFlags.images[0] == "-" {
				var imagesListData strings.Builder

				if _, err := io.Copy(&imagesListData, os.Stdin); err != nil {
					return fmt.Errorf("error reading from stdin: %w", err)
				}

				imageRegistryCreateCmdFlags.images = strings.Split(strings.Trim(imagesListData.String(), "\n"), "\n")
			}

			slog.Info("starting image mirror", "images", len(imageRegistryCreateCmdFlags.images))

			if err := artifacts.Mirror(ctx, options, imageRegistryCreateCmdFlags.images, addr); err != nil {
				return fmt.Errorf("error copying images: %w", err)
			}

			cancel()

			return nil
		})

		if err := eg.Wait(); err != nil {
			return err
		}

		return nil
	},
}

var imageRegistryCreateCmdFlags struct {
	debug  bool
	images []string
}

var imageRegistryServeCommand = &cobra.Command{
	Use:   "serve <path>",
	Short: "Serve images from a local storage",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		storePath := args[0]

		reg := olareg.New(config.Config{
			HTTP: config.ConfigHTTP{
				Addr:     imageRegistryServeCmdFlags.address,
				CertFile: imageRegistryServeCmdFlags.tlsCertFile,
				KeyFile:  imageRegistryServeCmdFlags.tlsKeyFile,
			},
			API: config.ConfigAPI{
				PushEnabled:   pointer.To(false),
				DeleteEnabled: pointer.To(false),
				Blob: config.ConfigAPIBlob{
					DeleteEnabled: pointer.To(false),
				},
			},
			Storage: config.ConfigStorage{
				StoreType: config.StoreDir,
				RootDir:   storePath,
				ReadOnly:  pointer.To(true),
				GC: config.ConfigGC{
					Frequency: -1, // disabled
				},
			},
			Log: slog.Default(),
		})

		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer cancel()

		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			slog.Info("Starting registry", "addr", imageRegistryServeCmdFlags.address, "path", storePath)

			return reg.Run(ctx)
		})

		eg.Go(func() error {
			<-ctx.Done()

			slog.Info("Shutting down")

			ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			return reg.Shutdown(ctx2)
		})

		return eg.Wait()
	},
}

var imageRegistryServeCmdFlags struct {
	address     string
	tlsCertFile string
	tlsKeyFile  string
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
	imageCacheCreateCmd.PersistentFlags().StringVar(&imageCacheCreateCmdFlags.platform, "platform", "linux/amd64", "platform to use for the cache")
	imageCacheCreateCmd.PersistentFlags().StringSliceVar(&imageCacheCreateCmdFlags.images, "images", nil, "images to cache")
	imageCacheCreateCmd.MarkPersistentFlagRequired("images") //nolint:errcheck
	imageCacheCreateCmd.PersistentFlags().BoolVar(&imageCacheCreateCmdFlags.insecure, "insecure", false, "allow insecure registries")
	imageCacheCreateCmd.PersistentFlags().BoolVar(&imageCacheCreateCmdFlags.force, "force", false, "force overwrite of existing image cache")

	imageCmd.AddCommand(imageIntegrationCmd)
	imageIntegrationCmd.PersistentFlags().StringVar(&imageIntegrationCmdFlags.installerTag, "installer-tag", "", "tag of the installer image to use")
	imageIntegrationCmd.MarkPersistentFlagRequired("installer-tag") //nolint:errcheck
	imageIntegrationCmd.PersistentFlags().StringVar(&imageIntegrationCmdFlags.registryAndUser, "registry-and-user", "", "registry and user to use for the images")
	imageIntegrationCmd.MarkPersistentFlagRequired("registry-and-user") //nolint:errcheck

	imageCmd.AddCommand(imageRegistryCommand)
	imageRegistryCommand.AddCommand(imageRegistryCreateCommand)
	imageRegistryCreateCommand.PersistentFlags().BoolVar(&imageRegistryCreateCmdFlags.debug, "debug", false, "enable debug logging")
	imageRegistryCreateCommand.PersistentFlags().StringSliceVar(&imageRegistryCreateCmdFlags.images, "images", nil, "images to cache")
	imageRegistryCreateCommand.MarkPersistentFlagRequired("images") //nolint:errcheck
	imageRegistryCommand.AddCommand(imageRegistryServeCommand)
	imageRegistryServeCommand.PersistentFlags().StringVar(&imageRegistryServeCmdFlags.address, "addr", ":5000", "address to serve the registry on")
	imageRegistryServeCommand.PersistentFlags().StringVar(&imageRegistryServeCmdFlags.tlsCertFile, "tls-cert-file", "", "path to TLS certificate file")
	imageRegistryServeCommand.PersistentFlags().StringVar(&imageRegistryServeCmdFlags.tlsKeyFile, "tls-key-file", "", "path to TLS key file")
}
