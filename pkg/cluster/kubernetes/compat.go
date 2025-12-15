// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"fmt"
	"sort"

	"github.com/blang/semver/v4"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
)

// NodeVersion holds the node identifier along with its Talos version.
type NodeVersion struct {
	Node    string
	Version *compatibility.TalosVersion
}

// GetNodesTalosVersions retrieves the Talos versions for the specified nodes.
func GetNodesTalosVersions(ctx context.Context, talosClient *client.Client, nodes []string) ([]NodeVersion, error) {
	eg, ctx := errgroup.WithContext(ctx)
	versions := make([]NodeVersion, len(nodes))

	for i, node := range nodes {
		eg.Go(func() error {
			nodeCtx := client.WithNode(ctx, node)

			resp, err := talosClient.Version(nodeCtx)
			if err != nil {
				return fmt.Errorf("node %q: %w", node, err)
			}

			v, err := compatibility.ParseTalosVersion(resp.Messages[0].GetVersion())
			if err != nil {
				return fmt.Errorf("node %q: %w", node, err)
			}

			versions[i] = NodeVersion{Node: node, Version: v}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return versions, nil
}

// GetMinimumTalosVersion returns the minimum Talos version from the provided list of NodeVersion.
func GetMinimumTalosVersion(versions []NodeVersion) (*compatibility.TalosVersion, error) {
	if len(versions) == 0 {
		return nil, nil
	}

	semvers := make([]struct {
		Index   int
		Version semver.Version
	}, len(versions))

	for i, nv := range versions {
		semverV, err := semver.ParseTolerant(nv.Version.String())
		if err != nil {
			return nil, fmt.Errorf("failed to parse talos version %q for node %q: %w", nv.Version.String(), nv.Node, err)
		}

		semvers[i] = struct {
			Index   int
			Version semver.Version
		}{
			Index:   i,
			Version: semverV,
		}
	}

	sort.Slice(semvers, func(i, j int) bool {
		return semvers[i].Version.Compare(semvers[j].Version) < 0
	})

	return versions[semvers[0].Index].Version, nil
}

// CheckCompatibility checks if all provided Talos versions are compatible with the given Kubernetes version.
func CheckCompatibility(k8sVersion string, versions []NodeVersion) error {
	parsedK8s, err := compatibility.ParseKubernetesVersion(k8sVersion)
	if err != nil {
		return err
	}

	for _, nv := range versions {
		if err := parsedK8s.SupportedWith(nv.Version); err != nil {
			return fmt.Errorf("node %q: %w", nv.Node, err)
		}
	}

	return nil
}

// VerifyVersionCompatibility retrieves Talos versions for the specified nodes and checks their compatibility with the given Kubernetes version.
func VerifyVersionCompatibility(ctx context.Context, talosClient *client.Client, nodes []string, k8sVersion string, logger func(string, ...any)) (
	*compatibility.TalosVersion, error,
) {
	versions, err := GetNodesTalosVersions(ctx, talosClient, nodes)
	if err != nil {
		return nil, err
	}

	if err := CheckCompatibility(k8sVersion, versions); err != nil {
		return nil, err
	}

	minV, err := GetMinimumTalosVersion(versions)
	if err != nil {
		return nil, err
	}

	for _, nv := range versions {
		logger("> %q: Talos version %s is compatible with Kubernetes version %s", nv.Node, nv.Version, k8sVersion)
	}

	return minV, nil
}
