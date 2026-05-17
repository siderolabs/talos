// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package mgmt

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/provision"
	remoteprovisionpb "github.com/siderolabs/talos/pkg/provision/api"
	"github.com/siderolabs/talos/pkg/provision/providers/qemu"
	"github.com/siderolabs/talos/pkg/provision/providers/remote"
)

var remoteProvisionLaunchCmdFlags struct {
	listen   string
	stateDir string
}

// remoteProvisionLaunchCmd is the gRPC server backing the remote
// provisioner. It wraps the in-process QEMU provisioner unchanged —
// Create/Destroy/Reflect delegate to it, with progress streamed back
// over the gRPC Create stream.
var remoteProvisionLaunchCmd = &cobra.Command{
	Use:   "remote-provision-launch",
	Short: "Run the remote QEMU provisioner gRPC server",
	Long: `Long-running gRPC daemon that wraps the in-process QEMU provisioner
so 'talosctl cluster create --remote-endpoint=...' can delegate to it.

The host must provide the full QEMU provisioner toolchain on PATH —
qemu-system-{amd64,arm64}, qemu-img, swtpm, virtiofsd, mkisofs — plus
OVMF firmware and access to /dev/kvm, /dev/net/tun and /dev/vhost-net.

Operators packaging their own image can layer talosctl on top of a
toolchain base, e.g.:

  FROM ghcr.io/siderolabs/build-container:<tag>
  COPY --from=ghcr.io/siderolabs/talosctl:<tag> /talosctl /usr/local/bin/talosctl
  ENTRYPOINT ["/usr/local/bin/talosctl", "remote-provision-launch"]
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRemoteProvision(cmd.Context(), remoteProvisionLaunchCmdFlags.listen, remoteProvisionLaunchCmdFlags.stateDir)
	},
}

func runRemoteProvision(ctx context.Context, listen, stateDir string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("remote-provision: create state dir %q: %w", stateDir, err)
	}

	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", listen)
	if err != nil {
		return fmt.Errorf("remote-provision: listen %q: %w", listen, err)
	}

	srv := grpc.NewServer()
	remoteprovisionpb.RegisterRemoteProvisionServiceServer(srv, &remoteProvisionImpl{stateDir: stateDir})

	go func() {
		<-ctx.Done()
		srv.GracefulStop()
	}()

	fmt.Fprintf(os.Stderr, "remote-provision listening on %s (state=%s)\n", listen, stateDir)

	return srv.Serve(lis)
}

// remoteProvisionImpl is the gRPC service implementation. It wraps the
// in-process QEMU provisioner: each Create/Destroy/Reflect constructs a
// short-lived qemu.Provisioner, delegates, and streams results back.
type remoteProvisionImpl struct {
	remoteprovisionpb.UnimplementedRemoteProvisionServiceServer

	stateDir string
}

// Ping returns server identity and architecture.
func (s *remoteProvisionImpl) Ping(context.Context, *emptypb.Empty) (*remoteprovisionpb.PingResponse, error) {
	return &remoteprovisionpb.PingResponse{
		ServerVersion: version.Tag,
		HostArch:      runtime.GOARCH,
	}, nil
}

// Create provisions a cluster by wrapping the in-process QEMU provisioner.
// Progress events from the provisioner's LogWriter are forwarded as
// streaming status events; the terminal event carries the JSON-encoded
// provision.Cluster.
func (s *remoteProvisionImpl) Create(req *remoteprovisionpb.CreateRequest, stream grpc.ServerStreamingServer[remoteprovisionpb.CreateEvent]) error {
	ctx := stream.Context()

	clusterReq, err := remote.UnmarshalClusterRequest(req.GetClusterRequest())
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "decode cluster request: %v", err)
	}

	// Rewrite any artifact paths supplied by the client to canonical
	// server-side paths (where rsync / UploadArtifact dropped them).
	applyArtifactPaths(&clusterReq, req.GetArtifactPaths())

	// Server owns the state directory.
	clusterReq.StateDirectory = s.stateDir

	// Server should run its own talosctl for sub-launches (loadbalancer,
	// qemu, dhcpd, ...). The client's SelfExecutable path is irrelevant.
	self, err := os.Executable()
	if err == nil {
		clusterReq.SelfExecutable = self
	}

	// Client-side CNI defaults (e.g. ~/.talos/cni) leak through the
	// ClusterRequest; rewrite them to server-local paths under stateDir
	// so the CNI bundle downloads + plugins land somewhere sane on the
	// server, not under the client's $HOME.
	clusterReq.Network.CNI.BinPath = []string{filepath.Join(s.stateDir, "cni", "bin")}
	clusterReq.Network.CNI.ConfDir = filepath.Join(s.stateDir, "cni", "conf.d")
	clusterReq.Network.CNI.CacheDir = filepath.Join(s.stateDir, "cni", "cache")

	logger := &streamLogWriter{stream: stream}

	// Boot assets passed as http(s) URLs (e.g. Image Factory ISO for
	// `create qemu --schematic-id`) are fetched server-side into the state
	// dir's cache — no need to ship them over the wire from the client.
	if err := downloadRequestAssets(ctx, &clusterReq, filepath.Join(s.stateDir, "cache"), logger); err != nil {
		return status.Errorf(codes.Internal, "download assets: %v", err)
	}

	provisionOpts, err := remote.UnmarshalOptions(req.GetOptions())
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "decode options: %v", err)
	}

	// LogWriter streams progress back to the client; TargetArch is the
	// server's (it runs the VMs). Appended last so they win.
	provisionOpts = append(provisionOpts,
		provision.WithLogWriter(logger),
		provision.WithTargetArch(runtime.GOARCH),
	)

	provisioner, err := qemu.NewProvisioner(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "qemu provisioner init: %v", err)
	}

	defer provisioner.Close() //nolint:errcheck

	cluster, err := provisioner.Create(ctx, clusterReq, provisionOpts...)
	if err != nil {
		return status.Errorf(codes.Internal, "qemu create: %v", err)
	}

	payload, err := remote.MarshalCluster(cluster)
	if err != nil {
		return status.Errorf(codes.Internal, "marshal cluster: %v", err)
	}

	return stream.Send(&remoteprovisionpb.CreateEvent{
		Event: &remoteprovisionpb.CreateEvent_Cluster{Cluster: payload},
	})
}

// Destroy tears down a cluster by name. Reflects against the local state
// directory to rehydrate the cluster, then delegates to the QEMU provisioner.
func (s *remoteProvisionImpl) Destroy(ctx context.Context, req *remoteprovisionpb.DestroyRequest) (*remoteprovisionpb.DestroyResponse, error) {
	provisioner, err := qemu.NewProvisioner(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "qemu provisioner init: %v", err)
	}

	defer provisioner.Close() //nolint:errcheck

	cluster, err := provisioner.Reflect(ctx, req.GetClusterName(), s.stateDir)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "reflect cluster %q: %v", req.GetClusterName(), err)
	}

	if err := provisioner.Destroy(ctx, cluster); err != nil {
		return nil, status.Errorf(codes.Internal, "destroy cluster %q: %v", req.GetClusterName(), err)
	}

	return &remoteprovisionpb.DestroyResponse{}, nil
}

// Reflect looks up a previously-created cluster on the server.
func (s *remoteProvisionImpl) Reflect(ctx context.Context, req *remoteprovisionpb.ReflectRequest) (*remoteprovisionpb.ReflectResponse, error) {
	provisioner, err := qemu.NewProvisioner(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "qemu provisioner init: %v", err)
	}

	defer provisioner.Close() //nolint:errcheck

	cluster, err := provisioner.Reflect(ctx, req.GetClusterName(), s.stateDir)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "reflect cluster %q: %v", req.GetClusterName(), err)
	}

	payload, err := remote.MarshalCluster(cluster)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal cluster: %v", err)
	}

	return &remoteprovisionpb.ReflectResponse{Cluster: payload}, nil
}

// applyArtifactPaths rewrites file-path fields of a ClusterRequest using
// the client's logical-name → server-canonical-path mapping. Unknown keys
// are ignored; empty values leave the original path untouched.
func applyArtifactPaths(req *provision.ClusterRequest, paths map[string]string) {
	set := func(dst *string, key string) {
		if v, ok := paths[key]; ok && v != "" {
			*dst = v
		}
	}

	set(&req.KernelPath, "kernel")
	set(&req.InitramfsPath, "initramfs")
	set(&req.ISOPath, "iso")
	set(&req.USBPath, "usb")
	set(&req.UKIPath, "uki")
	set(&req.DiskImagePath, "diskimage")
	set(&req.IPXEBootScript, "ipxe")
}

// streamLogWriter adapts an io.Writer to the gRPC Create stream: each
// non-empty line is emitted as a status event.
type streamLogWriter struct {
	stream grpc.ServerStreamingServer[remoteprovisionpb.CreateEvent]
	buf    bytes.Buffer
}

func (w *streamLogWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)

	for {
		idx := bytes.IndexByte(w.buf.Bytes(), '\n')
		if idx < 0 {
			break
		}

		line := w.buf.Next(idx + 1)
		line = bytes.TrimRight(line, "\r\n")

		if len(line) == 0 {
			continue
		}

		if err := w.stream.Send(&remoteprovisionpb.CreateEvent{
			Event: &remoteprovisionpb.CreateEvent_Status{Status: string(line)},
		}); err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

func init() {
	remoteProvisionLaunchCmd.Flags().StringVar(&remoteProvisionLaunchCmdFlags.listen, "listen", "0.0.0.0:50100", "address to listen on for gRPC")
	remoteProvisionLaunchCmd.Flags().StringVar(&remoteProvisionLaunchCmdFlags.stateDir, "state-dir", "/var/lib/talos-remote-provision", "directory for per-cluster state and artifact cache")
	addCommand(remoteProvisionLaunchCmd)
}
