// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package remote

import (
	"context"
	"fmt"

	"github.com/siderolabs/talos/pkg/provision"
	remoteprovisionpb "github.com/siderolabs/talos/pkg/provision/api"
)

var _ provision.RebootProvisioner = (*Provisioner)(nil)

// SyncBootArtifacts uploads and activates a cluster's kernel and initramfs.
func (p *Provisioner) SyncBootArtifacts(ctx context.Context, clusterName, kernelPath, initramfsPath string) (map[string]bool, error) {
	if _, err := p.Reflect(ctx, clusterName, ""); err != nil {
		return nil, err
	}

	client, err := p.dial(ctx)
	if err != nil {
		return nil, err
	}

	refs, err := p.uploadArtifacts(ctx, client, &provision.ClusterRequest{
		KernelPath:    kernelPath,
		InitramfsPath: initramfsPath,
	})
	if err != nil {
		return nil, fmt.Errorf("remote: upload boot artifacts: %w", err)
	}

	resp, err := client.SyncBootArtifacts(ctx, &remoteprovisionpb.SyncBootArtifactsRequest{
		ClusterName:   clusterName,
		ArtifactPaths: refs,
	})
	if err != nil {
		return nil, fmt.Errorf("remote: sync boot artifacts: %w", err)
	}

	return resp.GetChanged(), nil
}

// RebootNode forcefully reboots a node through the remote QEMU provisioner.
func (p *Provisioner) RebootNode(ctx context.Context, cluster provision.Cluster, node provision.NodeInfo) error {
	client, err := p.dial(ctx)
	if err != nil {
		return err
	}

	_, err = client.Reboot(ctx, &remoteprovisionpb.RebootRequest{
		ClusterName: cluster.Info().ClusterName,
		MachineName: node.Name,
	})
	if err != nil {
		return fmt.Errorf("remote: reboot node %q: %w", node.Name, err)
	}

	return nil
}
