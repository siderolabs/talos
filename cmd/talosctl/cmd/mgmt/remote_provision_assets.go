// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package mgmt

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-getter/v2"

	"github.com/siderolabs/talos/pkg/provision"
)

// downloadRequestAssets fetches any boot-asset fields of req that are http(s)
// URLs into cacheDir, rewriting the field to the resulting local path.
//
// This lets a remote client pass Image Factory URLs (e.g. for `create qemu
// --schematic-id`) and have the server fetch them directly, instead of
// shipping large artifacts over gRPC. Fields that are already local paths
// are left untouched.
func downloadRequestAssets(ctx context.Context, req *provision.ClusterRequest, cacheDir string, logw io.Writer) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("create cache dir %q: %w", cacheDir, err)
	}

	for _, asset := range []struct {
		path *string
		// disableArchive matches the client-side downloadBootAssets: the
		// Image Factory serves these compressed, and they must not be
		// auto-extracted by go-getter.
		disableArchive bool
	}{
		{path: &req.KernelPath},
		{path: &req.InitramfsPath, disableArchive: true},
		{path: &req.ISOPath},
		{path: &req.USBPath},
		{path: &req.UKIPath},
		{path: &req.DiskImagePath, disableArchive: true},
	} {
		if *asset.path == "" {
			continue
		}

		u, err := url.Parse(*asset.path)
		if err != nil || !(u.Scheme == "http" || u.Scheme == "https") {
			// not a URL — already a local path
			continue
		}

		destPath := filepath.Join(cacheDir, strings.NewReplacer("/", "-", ":", "-").Replace(u.String()))

		if _, err := os.Stat(destPath); err == nil {
			// already cached
			*asset.path = destPath

			continue
		}

		fmt.Fprintf(logw, "remote-provision: downloading asset %s\n", u.String())

		if asset.disableArchive {
			q := u.Query()
			q.Set("archive", "false")
			u.RawQuery = q.Encode()
		}

		client := getter.Client{
			Getters: []getter.Getter{
				&getter.HttpGetter{
					HeadFirstTimeout: 30 * time.Minute,
					ReadTimeout:      30 * time.Minute,
				},
			},
		}

		if _, err := client.Get(ctx, &getter.Request{
			Src:     u.String(),
			Dst:     destPath,
			GetMode: getter.ModeFile,
		}); err != nil {
			os.Remove(destPath) //nolint:errcheck

			return fmt.Errorf("download %s: %w", u.String(), err)
		}

		*asset.path = destPath
	}

	return nil
}
