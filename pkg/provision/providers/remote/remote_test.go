// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package remote_test

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/provision"
	remoteprovisionpb "github.com/siderolabs/talos/pkg/provision/api"
	"github.com/siderolabs/talos/pkg/provision/providers/remote"
)

// fakeServer is a minimal in-process gRPC server that exercises the wire
// shape of the remote provisioner.
type fakeServer struct {
	remoteprovisionpb.UnimplementedRemoteProvisionServiceServer

	mu            sync.Mutex
	synced        map[string]string
	rebootedNodes []string
}

func (*fakeServer) Ping(context.Context, *emptypb.Empty) (*remoteprovisionpb.PingResponse, error) {
	return &remoteprovisionpb.PingResponse{ServerVersion: "fake", HostArch: "amd64"}, nil
}

func (*fakeServer) Create(req *remoteprovisionpb.CreateRequest, stream grpc.ServerStreamingServer[remoteprovisionpb.CreateEvent]) error {
	var head struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal(req.GetClusterRequest(), &head); err != nil {
		return err
	}

	for _, s := range []string{"step 1", "step 2", "step 3"} {
		if err := stream.Send(&remoteprovisionpb.CreateEvent{Event: &remoteprovisionpb.CreateEvent_Status{Status: s}}); err != nil {
			return err
		}
	}

	payload, err := json.Marshal(map[string]any{
		"provisioner": remote.ProviderName,
		"state_path":  "/tmp/" + head.Name,
		"info": provision.ClusterInfo{
			ClusterName: head.Name,
			Network: provision.NetworkInfo{
				Name:         head.Name,
				CIDRs:        []netip.Prefix{netip.MustParsePrefix("10.5.0.0/24")},
				GatewayAddrs: []netip.Addr{netip.MustParseAddr("10.5.0.1")},
				MTU:          1500,
			},
			Nodes: []provision.NodeInfo{{
				ID:   "cp-1",
				Name: "cp-1",
				Type: machine.TypeControlPlane,
				IPs:  []netip.Addr{netip.MustParseAddr("10.5.0.2")},
			}},
			KubernetesEndpoint: "https://10.5.0.1:6443",
		},
	})
	if err != nil {
		return err
	}

	return stream.Send(&remoteprovisionpb.CreateEvent{Event: &remoteprovisionpb.CreateEvent_Cluster{Cluster: payload}})
}

func (*fakeServer) Destroy(_ context.Context, _ *remoteprovisionpb.DestroyRequest) (*remoteprovisionpb.DestroyResponse, error) {
	return &remoteprovisionpb.DestroyResponse{}, nil
}

func (*fakeServer) Reflect(_ context.Context, req *remoteprovisionpb.ReflectRequest) (*remoteprovisionpb.ReflectResponse, error) {
	payload, err := json.Marshal(map[string]any{
		"provisioner": remote.ProviderName,
		"state_path":  "/tmp/" + req.GetClusterName(),
		"info":        provision.ClusterInfo{ClusterName: req.GetClusterName()},
	})
	if err != nil {
		return nil, err
	}

	return &remoteprovisionpb.ReflectResponse{Cluster: payload}, nil
}

func (*fakeServer) StatArtifact(_ context.Context, req *remoteprovisionpb.StatArtifactRequest) (*remoteprovisionpb.StatArtifactResponse, error) {
	return &remoteprovisionpb.StatArtifactResponse{Exists: true, Path: "/cache/" + req.GetSha256()}, nil
}

func (s *fakeServer) SyncBootArtifacts(_ context.Context, req *remoteprovisionpb.SyncBootArtifactsRequest) (*remoteprovisionpb.SyncBootArtifactsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.synced = req.GetArtifactPaths()

	return &remoteprovisionpb.SyncBootArtifactsResponse{Changed: map[string]bool{
		"kernel":    true,
		"initramfs": true,
	}}, nil
}

func (s *fakeServer) Reboot(_ context.Context, req *remoteprovisionpb.RebootRequest) (*remoteprovisionpb.RebootResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rebootedNodes = append(s.rebootedNodes, req.GetMachineName())

	return &remoteprovisionpb.RebootResponse{}, nil
}

// startFakeServer spins up the in-process gRPC server, returning its
// endpoint and a teardown function.
func startFakeServer(t *testing.T) (string, *fakeServer, func()) {
	t.Helper()

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	fake := &fakeServer{}
	remoteprovisionpb.RegisterRemoteProvisionServiceServer(srv, fake)

	go func() {
		srv.Serve(lis) //nolint:errcheck
	}()

	return lis.Addr().String(), fake, func() {
		srv.GracefulStop()
	}
}

func TestProvisionerCreateRoundtrip(t *testing.T) {
	endpoint, _, stop := startFakeServer(t)
	defer stop()

	// Give the server a moment to be ready for Dial.
	time.Sleep(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := remote.NewProvisioner(ctx, endpoint)
	require.NoError(t, err)

	defer p.Close() //nolint:errcheck

	req := provision.ClusterRequest{
		Name: "phase2-roundtrip",
		Network: provision.NetworkRequest{
			Name:  "phase2-roundtrip",
			CIDRs: []netip.Prefix{netip.MustParsePrefix("10.5.0.0/24")},
		},
		Nodes: provision.NodeRequests{
			{Name: "cp-1", Type: machine.TypeControlPlane},
		},
	}

	cluster, err := p.Create(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, cluster)
	require.Equal(t, "phase2-roundtrip", cluster.Info().ClusterName)
	require.Equal(t, remote.ProviderName, cluster.Provisioner())

	require.NoError(t, p.Destroy(ctx, cluster))

	// Reflect roundtrip
	reflected, err := p.Reflect(ctx, "another-cluster", "")
	require.NoError(t, err)
	require.Equal(t, "another-cluster", reflected.Info().ClusterName)
}

func TestProvisionerSyncAndReboot(t *testing.T) {
	endpoint, server, stop := startFakeServer(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	provisioner, err := remote.NewProvisioner(ctx, endpoint)
	require.NoError(t, err)

	p := provisioner.(*remote.Provisioner)
	defer p.Close() //nolint:errcheck

	dir := t.TempDir()
	kernelPath := filepath.Join(dir, "vmlinuz-amd64")
	initramfsPath := filepath.Join(dir, "initramfs-amd64.xz")

	require.NoError(t, os.WriteFile(kernelPath, []byte("kernel"), 0o600))
	require.NoError(t, os.WriteFile(initramfsPath, []byte("initramfs"), 0o600))

	changed, err := p.SyncBootArtifacts(ctx, "test-cluster", kernelPath, initramfsPath)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"kernel": true, "initramfs": true}, changed)

	server.mu.Lock()
	syncedCount := len(server.synced)
	kernelRef := server.synced["kernel"]
	initramfsRef := server.synced["initramfs"]
	server.mu.Unlock()

	require.Equal(t, 2, syncedCount)
	require.Contains(t, kernelRef, "/cache/")
	require.Contains(t, initramfsRef, "/cache/")

	cluster, err := p.Reflect(ctx, "test-cluster", "")
	require.NoError(t, err)
	require.NoError(t, p.RebootNode(ctx, cluster, provision.NodeInfo{Name: "worker-1"}))

	server.mu.Lock()
	rebootedNodes := append([]string(nil), server.rebootedNodes...)
	server.mu.Unlock()

	require.Equal(t, []string{"worker-1"}, rebootedNodes)
}

func TestNewProvisionerEmptyEndpoint(t *testing.T) {
	_, err := remote.NewProvisioner(context.Background(), "")
	require.Error(t, err)
}

// insecureDial returns the gRPC dial option for Phase 2 tests. Currently
// unused outside the Provisioner itself but kept here for reference.
var _ = func() grpc.DialOption { return grpc.WithTransportCredentials(insecure.NewCredentials()) }
