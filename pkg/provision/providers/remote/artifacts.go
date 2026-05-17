// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package remote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/siderolabs/talos/pkg/provision"
	remoteprovisionpb "github.com/siderolabs/talos/pkg/provision/api"
)

// artifactUploadChunk is the data frame size for UploadArtifact streams.
const artifactUploadChunk = 1 << 20 // 1 MiB

// uploadArtifacts uploads every local-file boot asset referenced by req to
// the server's content-addressed cache, returning a logical-name → server-
// path map for CreateRequest.artifact_paths.
//
// URL assets are left alone — the server fetches those itself.
func (p *Provisioner) uploadArtifacts(ctx context.Context, client remoteprovisionpb.RemoteProvisionServiceClient, req *provision.ClusterRequest) (map[string]string, error) {
	assets := []struct {
		key  string
		path string
	}{
		{"kernel", req.KernelPath},
		{"initramfs", req.InitramfsPath},
		{"iso", req.ISOPath},
		{"usb", req.USBPath},
		{"uki", req.UKIPath},
		{"diskimage", req.DiskImagePath},
		{"ipxe", req.IPXEBootScript},
	}

	refs := map[string]string{}

	for _, a := range assets {
		if a.path == "" || isURL(a.path) {
			continue
		}

		serverPath, err := p.uploadArtifact(ctx, client, a.path)
		if err != nil {
			return nil, fmt.Errorf("%s artifact %q: %w", a.key, a.path, err)
		}

		refs[a.key] = serverPath
	}

	return refs, nil
}

// uploadArtifact uploads a single file, skipping the transfer if the
// server already has it cached. Returns the canonical server path.
//
//nolint:gocyclo
func (p *Provisioner) uploadArtifact(ctx context.Context, client remoteprovisionpb.RemoteProvisionServiceClient, path string) (string, error) {
	sha, err := sha256File(path)
	if err != nil {
		return "", err
	}

	if stat, err := client.StatArtifact(ctx, &remoteprovisionpb.StatArtifactRequest{Sha256: sha}); err == nil && stat.GetExists() {
		return stat.GetPath(), nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}

	defer f.Close() //nolint:errcheck

	stream, err := client.UploadArtifact(ctx)
	if err != nil {
		return "", err
	}

	// Header frame: the digest.
	if err := stream.Send(&remoteprovisionpb.ArtifactChunk{
		Payload: &remoteprovisionpb.ArtifactChunk_Sha256{Sha256: sha},
	}); err != nil {
		return "", err
	}

	buf := make([]byte, artifactUploadChunk)

	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			if err := stream.Send(&remoteprovisionpb.ArtifactChunk{
				Payload: &remoteprovisionpb.ArtifactChunk_Data{Data: buf[:n]},
			}); err != nil {
				return "", err
			}
		}

		if errors.Is(readErr, io.EOF) {
			break
		}

		if readErr != nil {
			return "", readErr
		}
	}

	ref, err := stream.CloseAndRecv()
	if err != nil {
		return "", err
	}

	return ref.GetPath(), nil
}

// sha256File returns the lowercase hex sha256 digest of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}

	defer f.Close() //nolint:errcheck

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// isURL reports whether s is an http(s) URL (an asset the server fetches
// itself) rather than a local file path.
func isURL(s string) bool {
	u, err := url.Parse(s)

	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}
