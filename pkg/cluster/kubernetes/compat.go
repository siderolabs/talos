// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
)

// VerifyVersionCompatibility checks if the given Kubernetes version is compatible with the current Talos version.
func VerifyVersionCompatibility(ctx context.Context, talosClient *client.Client, nodes []string, k8sVersion string, logger func(string, ...any)) error {
	eg, ctx := errgroup.WithContext(ctx)

	k8sVersionParsed, err := compatibility.ParseKubernetesVersion(k8sVersion)
	if err != nil {
		return fmt.Errorf("error parsing Kubernetes version %q: %w", k8sVersion, err)
	}

	for _, node := range nodes {
		eg.Go(func() error {
			nodeCtx := client.WithNode(ctx, node)

			versionResp, err := talosClient.Version(nodeCtx)
			if err != nil {
				return fmt.Errorf("error getting Talos version on node %q: %w", node, err)
			}

			talosVersion, err := compatibility.ParseTalosVersion(versionResp.Messages[0].GetVersion())
			if err != nil {
				return fmt.Errorf("error parsing Talos version on node %q: %w", node, err)
			}

			if err = k8sVersionParsed.SupportedWith(talosVersion); err != nil {
				return fmt.Errorf("compatibility check failed on node %q: %w", node, err)
			}

			logger("> %q: Talos version %s is compatible with Kubernetes version %s", node, talosVersion, k8sVersion)

			return nil
		})
	}

	return eg.Wait()
}
