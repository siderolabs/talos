// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package remote_test

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
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
}

func (fakeServer) Ping(context.Context, *emptypb.Empty) (*remoteprovisionpb.PingResponse, error) {
	return &remoteprovisionpb.PingResponse{ServerVersion: "fake"}, nil
}

func (fakeServer) Create(req *remoteprovisionpb.CreateRequest, stream grpc.ServerStreamingServer[remoteprovisionpb.CreateEvent]) error {
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

func (fakeServer) Destroy(_ context.Context, _ *remoteprovisionpb.DestroyRequest) (*remoteprovisionpb.DestroyResponse, error) {
	return &remoteprovisionpb.DestroyResponse{}, nil
}

func (fakeServer) Reflect(_ context.Context, req *remoteprovisionpb.ReflectRequest) (*remoteprovisionpb.ReflectResponse, error) {
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

// startFakeServer spins up the in-process gRPC server, returning its
// endpoint and a teardown function.
func startFakeServer(t *testing.T) (string, func()) {
	t.Helper()

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	remoteprovisionpb.RegisterRemoteProvisionServiceServer(srv, fakeServer{})

	go func() {
		srv.Serve(lis) //nolint:errcheck
	}()

	return lis.Addr().String(), func() {
		srv.GracefulStop()
	}
}

func TestProvisionerCreateRoundtrip(t *testing.T) {
	endpoint, stop := startFakeServer(t)
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

func TestNewProvisionerEmptyEndpoint(t *testing.T) {
	_, err := remote.NewProvisioner(context.Background(), "")
	require.Error(t, err)
}

// insecureDial returns the gRPC dial option for Phase 2 tests. Currently
// unused outside the Provisioner itself but kept here for reference.
var _ = func() grpc.DialOption { return grpc.WithTransportCredentials(insecure.NewCredentials()) }
