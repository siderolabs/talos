// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/go-getter/v2"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/pkg/cluster/check"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/access"
)

// downloadBootAssets downloads the boot assets in the given qemuOps if they are URLs, and replaces their URL paths with the downloaded paths on the filesystem.
//
// As it modifies the qemuOps struct, it needs to be passed by reference.
//
//nolint:gocyclo
func downloadBootAssets(ctx context.Context, qOps *clusterops.Qemu) error {
	// download & cache images if provides as URLs
	for _, downloadableImage := range []struct {
		path           *string
		disableArchive bool
	}{
		{
			path: &qOps.NodeVmlinuzPath,
		},
		{
			path:           &qOps.NodeInitramfsPath,
			disableArchive: true,
		},
		{
			path: &qOps.NodeISOPath,
		},
		{
			path: &qOps.NodeUSBPath,
		},
		{
			path: &qOps.NodeUKIPath,
		},
		{
			path: &qOps.NodeDiskImagePath,
			// we disable extracting the compressed image since we handle zstd for disk images
			disableArchive: true,
		},
	} {
		if *downloadableImage.path == "" {
			continue
		}

		u, err := url.Parse(*downloadableImage.path)
		if err != nil || !(u.Scheme == "http" || u.Scheme == "https") {
			// not a URL
			continue
		}

		defaultStateDir, err := clientconfig.GetTalosDirectory()
		if err != nil {
			return err
		}

		cacheDir := filepath.Join(defaultStateDir, "cache")

		if err = os.MkdirAll(cacheDir, 0o755); err != nil {
			return err
		}

		destPath := strings.ReplaceAll(
			strings.ReplaceAll(u.String(), "/", "-"),
			":", "-")

		_, err = os.Stat(filepath.Join(cacheDir, destPath))
		if err == nil {
			*downloadableImage.path = filepath.Join(cacheDir, destPath)

			// already cached
			continue
		}

		fmt.Fprintf(os.Stderr, "downloading asset from %q to %q\n", u.String(), filepath.Join(cacheDir, destPath))

		client := getter.Client{
			Getters: []getter.Getter{
				&getter.HttpGetter{
					HeadFirstTimeout: 30 * time.Minute,
					ReadTimeout:      30 * time.Minute,
				},
			},
		}

		if downloadableImage.disableArchive {
			q := u.Query()

			q.Set("archive", "false")

			u.RawQuery = q.Encode()
		}

		_, err = client.Get(ctx, &getter.Request{
			Src:     u.String(),
			Dst:     filepath.Join(cacheDir, destPath),
			GetMode: getter.ModeFile,
		})
		if err != nil {
			// clean up the destination on failure
			os.Remove(filepath.Join(cacheDir, destPath)) //nolint:errcheck

			return err
		}

		*downloadableImage.path = filepath.Join(cacheDir, destPath)
	}

	return nil
}

func postCreate(
	ctx context.Context,
	cOps clusterops.Common,
	bundleTalosconfig *clientconfig.Config,
	cluster provision.Cluster,
	provisionOptions []provision.Option,
	request provision.ClusterRequest,
) error {
	if err := saveConfig(bundleTalosconfig, cOps.TalosconfigDestination); err != nil {
		return err
	}

	clusterAccess := access.NewAdapter(cluster, provisionOptions...)
	defer clusterAccess.Close() //nolint:errcheck

	if cOps.ApplyConfigEnabled {
		err := clusterAccess.ApplyConfig(ctx, request.Nodes, request.SiderolinkRequest, os.Stdout)
		if err != nil {
			return err
		}
	}

	return bootstrapCluster(ctx, clusterAccess, cOps)
}

func bootstrapCluster(ctx context.Context, clusterAccess *access.Adapter, cOps clusterops.Common) error {
	if cOps.SkipInjectingConfig && !cOps.ApplyConfigEnabled {
		return nil
	}

	if !cOps.WithInitNode {
		if err := clusterAccess.Bootstrap(ctx, os.Stdout); err != nil {
			return fmt.Errorf("bootstrap error: %w", err)
		}
	}

	if !cOps.ClusterWait {
		return nil
	}

	// Run cluster readiness checks
	checkCtx, checkCtxCancel := context.WithTimeout(ctx, cOps.ClusterWaitTimeout)
	defer checkCtxCancel()

	checks := check.DefaultClusterChecks()

	if cOps.SkipK8sNodeReadinessCheck {
		checks = slices.Concat(check.PreBootSequenceChecks(), check.K8sComponentsReadinessChecks())
	}

	checks = slices.Concat(checks, check.ExtraClusterChecks())

	if err := check.Wait(checkCtx, clusterAccess, checks, check.StderrReporter()); err != nil {
		return err
	}

	if cOps.SkipKubeconfig {
		return nil
	}

	return mergeKubeconfig(ctx, clusterAccess)
}
