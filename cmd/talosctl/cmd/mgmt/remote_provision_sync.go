// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package mgmt

import (
	"context"
	"slices"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/provision"
	remoteprovisionpb "github.com/siderolabs/talos/pkg/provision/api"
	"github.com/siderolabs/talos/pkg/provision/providers/qemu"
)

// SyncBootArtifacts atomically updates the stable kernel and initramfs
// references used by an existing no-bootloader cluster.
func (s *remoteProvisionImpl) SyncBootArtifacts(ctx context.Context, req *remoteprovisionpb.SyncBootArtifactsRequest) (*remoteprovisionpb.SyncBootArtifactsResponse, error) {
	if len(req.GetArtifactPaths()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no boot artifacts supplied")
	}

	for key := range req.GetArtifactPaths() {
		if !isBootArtifactKey(key) {
			return nil, status.Errorf(codes.InvalidArgument, "unsupported boot artifact %q", key)
		}
	}

	_, provisioner, err := s.reflectQEMUCluster(ctx, req.GetClusterName())
	if err != nil {
		return nil, err
	}

	defer provisioner.Close() //nolint:errcheck

	_, changed, err := s.syncBootArtifactPaths(req.GetClusterName(), req.GetArtifactPaths())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "sync boot artifacts: %v", err)
	}

	return &remoteprovisionpb.SyncBootArtifactsResponse{Changed: changed}, nil
}

// Reboot forcefully restarts one QEMU node.
func (s *remoteProvisionImpl) Reboot(ctx context.Context, req *remoteprovisionpb.RebootRequest) (*remoteprovisionpb.RebootResponse, error) {
	cluster, provisioner, err := s.reflectQEMUCluster(ctx, req.GetClusterName())
	if err != nil {
		return nil, err
	}

	defer provisioner.Close() //nolint:errcheck

	nodeIndex := slices.IndexFunc(cluster.Info().Nodes, func(node provision.NodeInfo) bool {
		return node.Name == req.GetMachineName()
	})
	if nodeIndex < 0 {
		return nil, status.Errorf(codes.NotFound, "node %q not found in cluster %q", req.GetMachineName(), req.GetClusterName())
	}

	rebooter, ok := provisioner.(provision.RebootProvisioner)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "QEMU provisioner does not support rebooting nodes")
	}

	if err := rebooter.RebootNode(ctx, cluster, cluster.Info().Nodes[nodeIndex]); err != nil {
		return nil, status.Errorf(codes.Internal, "reboot node %q: %v", req.GetMachineName(), err)
	}

	return &remoteprovisionpb.RebootResponse{}, nil
}

func (s *remoteProvisionImpl) reflectQEMUCluster(ctx context.Context, clusterName string) (provision.Cluster, provision.Provisioner, error) {
	provisioner, err := qemu.NewProvisioner(ctx)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "qemu provisioner init: %v", err)
	}

	cluster, err := provisioner.Reflect(ctx, clusterName, s.stateDir)
	if err != nil {
		provisioner.Close() //nolint:errcheck

		return nil, nil, status.Errorf(codes.NotFound, "reflect cluster %q: %v", clusterName, err)
	}

	return cluster, provisioner, nil
}
