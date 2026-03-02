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
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/blang/semver/v4"
	"github.com/dustin/go-humanize"
	"github.com/siderolabs/gen/ensure"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/multiplex"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/pull"
	mgmthelpers "github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/artifacts"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services/registry"
	"github.com/siderolabs/talos/pkg/imager/cache"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/reporter"
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

func (flags imageCmdFlagsType) containerdInstance() (*common.ContainerdInstance, error) {
	switch flags.namespace {
	case "cri":
		return &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CRI,
			Namespace: common.ContainerdNamespace_NS_CRI,
		}, nil
	case "system":
		return &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CRI,
			Namespace: common.ContainerdNamespace_NS_SYSTEM,
		}, nil
	case "inmem":
		return &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CONTAINERD,
			Namespace: common.ContainerdNamespace_NS_SYSTEM,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported namespace %q", flags.namespace)
	}
}

// imagesCmd represents the image command.
var imageCmd = &cobra.Command{
	Use:     "image",
	Aliases: []string{"images"},
	Short:   "Manage container images",
	Long:    ``,
	Args:    cobra.NoArgs,
}

// imageListCmd represents the image list command.
var imageListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l", "ls"},
	Short:   "List images in the machine's container runtime",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return imageList()
	},
}

func imageList() error {
	return WithClientAndNodes(func(ctx context.Context, c *client.Client, nodes []string) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		containerdInstance, err := imageCmdFlags.containerdInstance()
		if err != nil {
			return err
		}

		responseChan := multiplex.Streaming(ctx, nodes,
			func(ctx context.Context) (grpc.ServerStreamingClient[machine.ImageServiceListResponse], error) {
				return c.ImageClient.List(ctx, &machine.ImageServiceListRequest{
					Containerd: containerdInstance,
				})
			},
		)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		headerWritten := false

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				if status.Code(resp.Err) == codes.Unimplemented {
					// fallback to legacy API for older Talos
					return imageListLegacy()
				}

				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

				continue
			}

			if !headerWritten {
				headerWritten = true

				fmt.Fprintln(w, "NODE\tIMAGE\tDIGEST\tSIZE\tCREATED")
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				resp.Node,
				resp.Payload.GetName(),
				resp.Payload.GetDigest(),
				humanize.Bytes(uint64(resp.Payload.GetSize())),
				resp.Payload.GetCreatedAt().AsTime().Format(time.RFC3339),
			)
		}

		return errors.Join(errs, w.Flush())
	})
}

// imageListLegacy lists images using the legacy ImageList API.
//
// Note: remove me in Talos 1.15.
func imageListLegacy() error {
	return WithClient(func(ctx context.Context, c *client.Client) error {
		ns, err := imageCmdFlags.apiNamespace()
		if err != nil {
			return err
		}

		rcv, err := c.ImageList(ctx, ns) //nolint:staticcheck // legacy talosctl methods, to be removed in Talos 1.15
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
}

// imagePullCmd represents the image pull command.
var imagePullCmd = &cobra.Command{
	Use:     "pull <image>",
	Aliases: []string{"p"},
	Short:   "Pull an image into the machine's container runtime",
	Long:    ``,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return imagePull(args[0])
	},
}

// imagePull pulls an image using modern API and showing progress.
func imagePull(imageRef string) error {
	return WithClientAndNodes(func(ctx context.Context, c *client.Client, nodes []string) error {
		rep := reporter.New()

		containerdInstance, err := imageCmdFlags.containerdInstance()
		if err != nil {
			return err
		}

		_, err = imagePullInternal(ctx, c, containerdInstance, nodes, imageRef, rep)

		return err
	})
}

func imagePullInternal(
	ctx context.Context,
	c *client.Client,
	containerdInstance *common.ContainerdInstance,
	nodes []string,
	imageRef string,
	rep *reporter.Reporter,
) (map[string]string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	responseChan := multiplex.Streaming(ctx, nodes,
		func(ctx context.Context) (grpc.ServerStreamingClient[machine.ImageServicePullResponse], error) {
			return c.ImageClient.Pull(ctx, &machine.ImageServicePullRequest{
				Containerd: containerdInstance,
				ImageRef:   imageRef,
			})
		},
	)

	finishedPulls := map[string]string{}

	var (
		w    pull.ProgressWriter
		errs error
	)

	for resp := range responseChan {
		if resp.Err != nil {
			if status.Code(resp.Err) == codes.Unimplemented {
				// fallback to legacy API for older Talos
				return nil, imagePullLegacy(imageRef)
			}

			errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

			continue
		}

		switch payload := resp.Payload.Response.(type) {
		case *machine.ImageServicePullResponse_PullProgress:
			if !rep.IsColorized() {
				// don't show progress if not colorized/terminal
				continue
			}

			w.UpdateJob(resp.Node, payload.PullProgress.GetLayerId(), payload.PullProgress.GetProgress())

			w.PrintLayerProgress(rep)
		case *machine.ImageServicePullResponse_Name:
			finishedPulls[resp.Node] = payload.Name
		}
	}

	if len(finishedPulls) > 0 {
		var sb strings.Builder

		for node, imageName := range finishedPulls {
			fmt.Fprintf(&sb, "%s: pulled image %s\n", node, imageName)
		}

		rep.Report(reporter.Update{
			Message: sb.String(),
			Status:  reporter.StatusSucceeded,
		})
	}

	return finishedPulls, errs
}

// imagePullLegacy pulls an image using the legacy ImagePull API.
//
// Note: remove me in Talos 1.15.
func imagePullLegacy(imageRef string) error {
	return WithClient(func(ctx context.Context, c *client.Client) error {
		ns, err := imageCmdFlags.apiNamespace()
		if err != nil {
			return err
		}

		err = c.ImagePull(ctx, ns, imageRef) //nolint:staticcheck // legacy talosctl methods, to be removed in Talos 1.15
		if err != nil {
			return fmt.Errorf("error pulling image: %w", err)
		}

		return nil
	})
}

// imageImportInternal imports an image from a tarball.
//
// Note: this is not exposed as a command, but used in talosctl debug flow.
//
//nolint:gocyclo
func imageImportInternal(
	ctx context.Context,
	c *client.Client,
	containerdInstance *common.ContainerdInstance,
	node string,
	imageTarballPath string,
	rep *reporter.Reporter,
) (string, error) {
	in, err := os.Open(imageTarballPath)
	if err != nil {
		return "", fmt.Errorf("failed to open image tarball: %w", err)
	}

	defer in.Close() //nolint:errcheck

	ctx = client.WithNode(ctx, node)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	rcv, err := c.ImageClient.Import(ctx)
	if err != nil {
		return "", err
	}

	if err = rcv.Send(&machine.ImageServiceImportRequest{
		Request: &machine.ImageServiceImportRequest_Containerd{
			Containerd: containerdInstance,
		},
	}); err != nil {
		return "", err
	}

	const chunkSize = 32 * 1024

	buf := make([]byte, chunkSize)

	var bytesImported uint64

	for {
		n, err := in.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}

			return "", fmt.Errorf("error reading image tarball: %w", err)
		}

		if n > 0 {
			if err = rcv.Send(&machine.ImageServiceImportRequest{
				Request: &machine.ImageServiceImportRequest_ImageChunk{
					ImageChunk: &common.Data{
						Bytes: buf[:n],
					},
				},
			}); err != nil {
				return "", fmt.Errorf("error sending image chunk: %w", err)
			}
		}

		bytesImported += uint64(n)

		if rep.IsColorized() {
			rep.Report(reporter.Update{
				Message: fmt.Sprintf("%s: %s imported", node, humanize.IBytes(bytesImported)),
				Status:  reporter.StatusRunning,
			})
		}
	}

	resp, err := rcv.CloseAndRecv()
	if err != nil {
		return "", fmt.Errorf("error closing send stream: %w", err)
	}

	rep.Report(reporter.Update{
		Message: fmt.Sprintf("%s: image imported %s from %s", node, resp.GetName(), imageTarballPath),
		Status:  reporter.StatusSucceeded,
	})

	return resp.GetName(), nil
}

// imageRemoveCmd represents the image remove command.
var imageRemoveCmd = &cobra.Command{
	Use:     "remove <image>",
	Aliases: []string{"rm"},
	Short:   "Remove an image from the machine's container runtime",
	Long:    ``,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return imageRemove(args[0])
	},
}

// imageRemove removes an image using modern API.
func imageRemove(imageRef string) error {
	return WithClientAndNodes(func(ctx context.Context, c *client.Client, nodes []string) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		containerdInstance, err := imageCmdFlags.containerdInstance()
		if err != nil {
			return err
		}

		responseChan := multiplex.Unary(ctx, nodes,
			func(ctx context.Context) (*emptypb.Empty, error) {
				return c.ImageClient.Remove(ctx, &machine.ImageServiceRemoveRequest{
					Containerd: containerdInstance,
					ImageRef:   imageRef,
				})
			},
		)

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))
			}
		}

		return errs
	})
}

var imageK8sBundleCmdFlags = struct {
	k8sVersion                 pflag.Value
	flannelVersion             pflag.Value
	corednsVersion             pflag.Value
	etcdVersion                pflag.Value
	kubeNetworkPoliciesVersion pflag.Value
}{
	k8sVersion:                 helpers.Semver(constants.DefaultKubernetesVersion),
	flannelVersion:             helpers.Semver(constants.FlannelVersion),
	corednsVersion:             helpers.Semver(constants.DefaultCoreDNSVersion),
	etcdVersion:                helpers.Semver(constants.DefaultEtcdVersion),
	kubeNetworkPoliciesVersion: helpers.Semver(constants.KubeNetworkPoliciesVersion),
}

// imageK8sBundleCmd represents the image k8s-bundle command.
var imageK8sBundleCmd = &cobra.Command{
	Use:     "k8s-bundle",
	Aliases: []string{"default"},
	Short:   "List the default Kubernetes images used by Talos",
	Long:    ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		images := images.ListWithOptions(container.NewV1Alpha1(
			&v1alpha1.Config{
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
			}),
			images.VersionsListOptions{
				KubernetesVersion:          imageK8sBundleCmdFlags.k8sVersion.String(),
				EtcdVersion:                imageK8sBundleCmdFlags.etcdVersion.String(),
				FlannelVersion:             imageK8sBundleCmdFlags.flannelVersion.String(),
				CoreDNSVersion:             imageK8sBundleCmdFlags.corednsVersion.String(),
				KubeNetworkPoliciesVersion: imageK8sBundleCmdFlags.kubeNetworkPoliciesVersion.String(),
			},
		)

		fmt.Printf("%s\n", images.Flannel)
		fmt.Printf("%s\n", images.CoreDNS)
		fmt.Printf("%s\n", images.Etcd)
		fmt.Printf("%s\n", images.Pause)
		fmt.Printf("%s\n", images.KubeAPIServer)
		fmt.Printf("%s\n", images.KubeControllerManager)
		fmt.Printf("%s\n", images.KubeScheduler)
		fmt.Printf("%s\n", images.KubeProxy)
		fmt.Printf("%s\n", images.Kubelet)
		fmt.Printf("%s\n", images.KubeNetworkPolicies)

		return nil
	},
}

var imageTalosBundleCmdFlags = struct {
	extensions bool
	overlays   bool
}{}

// imageTalosBundleCmd represents the image talos-bundle command.
var imageTalosBundleCmd = &cobra.Command{
	Use:   "talos-bundle [talos-version]",
	Short: "List the default system images and extensions used for Talos",
	Long:  ``,
	Args: cobra.MatchAll(
		cobra.RangeArgs(0, 1),
		func(cmd *cobra.Command, args []string) error {
			maximumVersion, err := semver.ParseTolerant(version.Tag)
			if err != nil {
				panic(err) // panic, this should never happen
			}

			maximumVersion.Patch = 0

			maximumVersion.Pre = nil
			if err := maximumVersion.IncrementMinor(); err != nil {
				panic(err) // panic, this should never happen
			}

			// If no version specified, use current version
			if len(args) == 0 {
				return nil
			}

			tag := args[0]

			if !strings.HasPrefix(tag, "v") {
				return fmt.Errorf("invalid tag %q: must have \"v\" prefix", tag)
			}

			ver, err := semver.ParseTolerant(tag)
			if err != nil {
				return fmt.Errorf("invalid argument %q: tag must be a valid semver", tag)
			}

			if !ver.GTE(minimumVersion) || !ver.LT(maximumVersion) {
				return fmt.Errorf(
					"invalid tag %q: must be between v%s (inclusive) and v%s (exclusive)", tag, minimumVersion, maximumVersion,
				)
			}

			return nil
		},
	),
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			tag        string
			err        error
			extensions []artifacts.ExtensionRef
			overlays   []artifacts.OverlayRef
		)

		// Default to current version if not specified
		if len(args) == 0 {
			tag = version.Tag
		} else {
			tag = args[0]
		}

		sources := images.ListSourcesFor(tag)

		fmt.Printf("%s\n", sources.Installer)
		fmt.Printf("%s\n", sources.InstallerBase)
		fmt.Printf("%s\n", sources.Imager)
		fmt.Printf("%s\n", sources.Talos)
		fmt.Printf("%s\n", sources.TalosctlAll)
		fmt.Printf("%s\n", sources.Overlays)
		fmt.Printf("%s\n", sources.Extensions)

		digestedReferences := []string{}

		if imageTalosBundleCmdFlags.extensions {
			extensions, err = artifacts.FetchOfficialExtensions(tag)
			if err != nil {
				return fmt.Errorf("error fetching official extensions for %s: %w", tag, err)
			}

			for _, extension := range extensions {
				digestedReferences = append(digestedReferences, fmt.Sprintf("%s@%s", extension.TaggedReference.String(), extension.Digest))
			}
		}

		if imageTalosBundleCmdFlags.overlays {
			overlays, err = artifacts.FetchOfficialOverlays(tag)
			if err != nil {
				return fmt.Errorf("error fetching official overlays for %s: %w", tag, err)
			}

			for _, overlay := range overlays {
				digestedReferences = append(digestedReferences, fmt.Sprintf("%s@%s", overlay.TaggedReference.String(), overlay.Digest))
			}
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
			imgs.Flannel.String(),
			imgs.CoreDNS.String(),
			imgs.Etcd.String(),
			imgs.KubeAPIServer.String(),
			imgs.KubeControllerManager.String(),
			imgs.KubeScheduler.String(),
			imgs.KubeProxy.String(),
			imgs.Kubelet.String(),
			imgs.Pause.String(),
			imgs.KubeNetworkPolicies.String(),
			"registry.k8s.io/conformance:v" + constants.DefaultKubernetesVersion,
			"docker.io/library/alpine:latest",
			"ghcr.io/siderolabs/talosctl:latest",
			"registry.k8s.io/kube-apiserver:v1.27.0",
			"registry.k8s.io/kube-apiserver:v1.27.1",
			"docker.io/library/alpine:3.23",
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

		if err = generateMirrorsConfigPatch(
			imageCacheServeCmdFlags.address,
			imageCacheServeCmdFlags.mirrors,
			imageCacheServeCmdFlags.tlsCertFile != "" && imageCacheServeCmdFlags.tlsKeyFile != "",
		); err != nil {
			development.Error("failed to generate Talos config patch for registry mirrors", zap.Error(err))
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

//nolint:gocyclo
func generateMirrorsConfigPatch(addr string, mirrors []string, secure bool) error {
	addresses := []string{}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	if host == "" || host == "0.0.0.0" || host == "[::]" {
		// list all IPs for the host
		ips, err := net.InterfaceAddrs()
		if err != nil {
			return err
		}

		for _, addr := range ips {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.IsLoopback() {
					continue
				}

				addresses = append(addresses, net.JoinHostPort(ipnet.IP.String(), port))
			}

			if ipa, ok := addr.(*net.IPAddr); ok {
				if ipa.IP.IsLoopback() {
					continue
				}

				addresses = append(addresses, net.JoinHostPort(ipa.IP.String(), port))
			}
		}
	} else {
		addresses = []string{net.JoinHostPort(host, port)}
	}

	if port == "0" {
		return nil // we do not generate patch for dynamic ports
	}

	patches := make([]config.Document, 0, len(mirrors))

	prefix := "http://"
	if secure {
		prefix = "https://"
	}

	for _, mirror := range mirrors {
		patch := cri.NewRegistryMirrorConfigV1Alpha1(mirror)
		patch.RegistryEndpoints = []cri.RegistryEndpoint{}

		for _, endpoint := range addresses {
			patch.RegistryEndpoints = append(patch.RegistryEndpoints, cri.RegistryEndpoint{
				EndpointURL: meta.URL{URL: ensure.Value(url.Parse(prefix + endpoint))},
			})
		}

		patches = append(patches, patch)
	}

	ctr, err := container.New(patches...)
	if err != nil {
		return err
	}

	patchBytes, err := ctr.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return err
	}

	const patchFile = "image-cache-mirrors-patch.yaml"

	log.Printf("writing config patch to %s", patchFile)

	return os.WriteFile(patchFile, patchBytes, 0o644)
}

var imageCacheServeCmdFlags struct {
	imageCachePath string
	address        string
	mirrors        []string
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
		caPEM, certPEM, keyPEM, err := mgmthelpers.GenerateSelfSignedCert(
			imageCacheCertGenCmdFlags.advertisedAddresses,
			imageCacheCertGenCmdFlags.advertisedNames,
		)
		if err != nil {
			return nil
		}

		if err = generateCAConfigPatch(caPEM); err != nil {
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

func generateCAConfigPatch(caPEM []byte) error {
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
	advertisedAddresses []net.IP
	advertisedNames     []string
	tlsCaFile           string
	tlsCertFile         string
	tlsKeyFile          string
}

func init() {
	imageCmd.PersistentFlags().StringVar(&imageCmdFlags.namespace, "namespace", "cri",
		"namespace to use: \"system\" (etcd and kubelet images), \"cri\" for all Kubernetes workloads, \"inmem\" for in-memory containerd instance",
	)
	addCommand(imageCmd)

	imageCmd.AddCommand(imageListCmd)
	imageCmd.AddCommand(imagePullCmd)
	imageCmd.AddCommand(imageRemoveCmd)

	imageCmd.AddCommand(imageTalosBundleCmd)
	imageTalosBundleCmd.PersistentFlags().BoolVar(&imageTalosBundleCmdFlags.overlays, "overlays", true, "Include images that belong to Talos overlays")
	imageTalosBundleCmd.PersistentFlags().BoolVar(&imageTalosBundleCmdFlags.extensions, "extensions", true, "Include images that belong to Talos extensions")

	imageCmd.AddCommand(imageK8sBundleCmd)
	imageK8sBundleCmd.PersistentFlags().Var(imageK8sBundleCmdFlags.k8sVersion, "k8s-version", "Kubernetes semantic version")
	imageK8sBundleCmd.PersistentFlags().Var(imageK8sBundleCmdFlags.etcdVersion, "etcd-version", "ETCD semantic version")
	imageK8sBundleCmd.PersistentFlags().Var(imageK8sBundleCmdFlags.flannelVersion, "flannel-version", "Flannel CNI semantic version")
	imageK8sBundleCmd.PersistentFlags().Var(imageK8sBundleCmdFlags.corednsVersion, "coredns-version", "CoreDNS semantic version")
	imageK8sBundleCmd.PersistentFlags().Var(imageK8sBundleCmdFlags.kubeNetworkPoliciesVersion, "kube-network-policies-version", "kube-network-policies semantic version")

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
	imageCacheServeCmd.PersistentFlags().StringSliceVar(&imageCacheServeCmdFlags.mirrors, "mirror", []string{"docker.io", "ghcr.io", "registry.k8s.io"},
		"list of registry mirrors to add to the Talos config patch")
	imageCacheServeCmd.PersistentFlags().StringVar(&imageCacheServeCmdFlags.tlsCertFile, "tls-cert-file", "", "TLS certificate file to use for serving")
	imageCacheServeCmd.PersistentFlags().StringVar(&imageCacheServeCmdFlags.tlsKeyFile, "tls-key-file", "", "TLS key file to use for serving")

	imageCmd.AddCommand(imageCacheCertGenCmd)
	imageCacheCertGenCmd.PersistentFlags().StringVar(&imageCacheCertGenCmdFlags.tlsCaFile, "tls-ca-file", "ca.crt", "TLS certificate authority file")
	imageCacheCertGenCmd.PersistentFlags().StringVar(&imageCacheCertGenCmdFlags.tlsCertFile, "tls-cert-file", "tls.crt", "TLS certificate file to use for serving")
	imageCacheCertGenCmd.PersistentFlags().StringVar(&imageCacheCertGenCmdFlags.tlsKeyFile, "tls-key-file", "tls.key", "TLS key file to use for serving")
	imageCacheCertGenCmd.PersistentFlags().IPSliceVar(&imageCacheCertGenCmdFlags.advertisedAddresses, "advertised-address", []net.IP{}, "The addresses to advertise.")
	imageCacheCertGenCmd.PersistentFlags().StringSliceVar(&imageCacheCertGenCmdFlags.advertisedNames, "advertised-name", []string{}, "The DNS names to advertise.")
	imageIntegrationCmd.MarkPersistentFlagRequired("advertised-address") //nolint:errcheck

	imageCmd.AddCommand(imageIntegrationCmd)
	imageIntegrationCmd.PersistentFlags().StringVar(&imageIntegrationCmdFlags.installerTag, "installer-tag", "", "tag of the installer image to use")
	imageIntegrationCmd.MarkPersistentFlagRequired("installer-tag") //nolint:errcheck
	imageIntegrationCmd.PersistentFlags().StringVar(&imageIntegrationCmdFlags.talosTag, "talos-tag", version.Tag, "tag of the installer image to use")
	imageIntegrationCmd.PersistentFlags().StringVar(&imageIntegrationCmdFlags.registryAndUser, "registry-and-user", "", "registry and user to use for the images")
	imageIntegrationCmd.MarkPersistentFlagRequired("registry-and-user") //nolint:errcheck
}
