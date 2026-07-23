// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package mgmt

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	remoteprovisionpb "github.com/siderolabs/talos/pkg/provision/api"
)

// artifactCacheDir is where uploaded, content-addressed boot artifacts live.
func (s *remoteProvisionImpl) artifactCacheDir() string {
	return filepath.Join(s.stateDir, "cache", "by-sha256")
}

func (s *remoteProvisionImpl) openBootArtifactRoot() (*os.Root, error) {
	dir := filepath.Join(s.stateDir, "boot-artifacts")

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	return os.OpenRoot(dir)
}

func (s *remoteProvisionImpl) syncBootArtifactPaths(clusterName string, paths map[string]string) (map[string]string, map[string]bool, error) {
	resolved := make(map[string]string, len(paths))
	changed := make(map[string]bool, len(paths))

	for key, path := range paths {
		if isBootArtifactKey(key) {
			if err := s.validateCachedArtifactPath(path); err != nil {
				return nil, nil, fmt.Errorf("invalid %s artifact: %w", key, err)
			}
		}
	}

	for key, path := range paths {
		resolved[key] = path

		if !isBootArtifactKey(key) {
			continue
		}

		stablePath, artifactChanged, err := s.syncBootArtifact(clusterName, key, path)
		if err != nil {
			return nil, nil, err
		}

		resolved[key] = stablePath
		changed[key] = artifactChanged
	}

	return resolved, changed, nil
}

func (s *remoteProvisionImpl) syncBootArtifact(clusterName, key, artifactPath string) (string, bool, error) {
	if !isBootArtifactKey(key) {
		return "", false, fmt.Errorf("unsupported boot artifact %q", key)
	}

	if err := s.validateCachedArtifactPath(artifactPath); err != nil {
		return "", false, fmt.Errorf("invalid %s artifact: %w", key, err)
	}

	if clusterName == "" || clusterName == "." {
		return "", false, fmt.Errorf("invalid cluster name %q", clusterName)
	}

	root, err := s.openBootArtifactRoot()
	if err != nil {
		return "", false, fmt.Errorf("open boot artifact root: %w", err)
	}

	defer root.Close() //nolint:errcheck

	if err := root.MkdirAll(clusterName, 0o755); err != nil {
		return "", false, fmt.Errorf("create boot artifact directory: %w", err)
	}

	clusterRoot, err := root.OpenRoot(clusterName)
	if err != nil {
		return "", false, fmt.Errorf("open cluster boot artifact directory: %w", err)
	}

	defer clusterRoot.Close() //nolint:errcheck

	stablePath := filepath.Join(clusterRoot.Name(), key)

	changed, err := replaceFile(clusterRoot, key, artifactPath)
	if err != nil {
		return "", false, fmt.Errorf("replace boot artifact: %w", err)
	}

	return stablePath, changed, nil
}

func replaceFile(root *os.Root, path, sourcePath string) (bool, error) {
	if info, err := root.Lstat(path); err == nil && info.Mode().IsRegular() {
		currentSHA, hashErr := fileSHA256(root, path)
		if hashErr == nil && currentSHA == filepath.Base(sourcePath) {
			return false, nil
		}
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return false, fmt.Errorf("open source: %w", err)
	}

	defer source.Close() //nolint:errcheck

	tmp, tmpPath, err := createTempFile(root)
	if err != nil {
		return false, fmt.Errorf("create temporary file: %w", err)
	}

	defer root.Remove(tmpPath) //nolint:errcheck
	defer tmp.Close()          //nolint:errcheck

	if _, err := io.Copy(tmp, source); err != nil {
		return false, fmt.Errorf("copy artifact: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return false, fmt.Errorf("close temporary file: %w", err)
	}

	if err := root.Rename(tmpPath, path); err != nil {
		return false, err
	}

	return true, nil
}

func createTempFile(root *os.Root) (*os.File, string, error) {
	for range 100 {
		var suffix [8]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return nil, "", err
		}

		name := fmt.Sprintf(".boot-artifact-%x.tmp", suffix)

		f, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return f, name, nil
		}

		if !errors.Is(err, fs.ErrExist) {
			return nil, "", err
		}
	}

	return nil, "", errors.New("failed to allocate temporary boot artifact file")
}

func fileSHA256(root *os.Root, path string) (string, error) {
	f, err := root.Open(path)
	if err != nil {
		return "", err
	}

	defer f.Close() //nolint:errcheck

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (s *remoteProvisionImpl) validateCachedArtifactPath(path string) error {
	sha := filepath.Base(path)
	if err := validateSHA256(sha); err != nil {
		return err
	}

	expected := filepath.Join(s.artifactCacheDir(), sha)
	if filepath.Clean(path) != expected {
		return fmt.Errorf("path %q is outside the artifact cache", path)
	}

	info, err := os.Stat(expected)
	if err != nil {
		return fmt.Errorf("stat cached artifact: %w", err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("cached artifact %q is not a regular file", expected)
	}

	return nil
}

func isBootArtifactKey(key string) bool {
	return key == "kernel" || key == "initramfs"
}

// StatArtifact reports whether a content-addressed artifact is already
// cached, letting the client skip a redundant upload.
func (s *remoteProvisionImpl) StatArtifact(_ context.Context, req *remoteprovisionpb.StatArtifactRequest) (*remoteprovisionpb.StatArtifactResponse, error) {
	sha := req.GetSha256()
	if err := validateSHA256(sha); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	path := filepath.Join(s.artifactCacheDir(), sha)

	if _, err := os.Stat(path); err == nil {
		return &remoteprovisionpb.StatArtifactResponse{Exists: true, Path: path}, nil
	}

	return &remoteprovisionpb.StatArtifactResponse{Exists: false}, nil
}

// UploadArtifact receives a boot artifact into the content-addressed cache.
// The first frame carries the sha256; the rest carry data. The server
// verifies the digest before committing the file.
//
//nolint:gocyclo
func (s *remoteProvisionImpl) UploadArtifact(stream grpc.ClientStreamingServer[remoteprovisionpb.ArtifactChunk, remoteprovisionpb.ArtifactRef]) error {
	first, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "upload: receiving header frame: %v", err)
	}

	sha := first.GetSha256()
	if err := validateSHA256(sha); err != nil {
		return status.Errorf(codes.InvalidArgument, "%v", err)
	}

	cacheDir := s.artifactCacheDir()
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return status.Errorf(codes.Internal, "create cache dir: %v", err)
	}

	finalPath := filepath.Join(cacheDir, sha)

	// Already cached — drain nothing further, just acknowledge.
	if _, err := os.Stat(finalPath); err == nil {
		return stream.SendAndClose(&remoteprovisionpb.ArtifactRef{Path: finalPath})
	}

	tmp, err := os.CreateTemp(cacheDir, sha+".*.partial")
	if err != nil {
		return status.Errorf(codes.Internal, "create temp file: %v", err)
	}

	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) //nolint:errcheck // no-op once renamed

	hasher := sha256.New()
	sink := io.MultiWriter(tmp, hasher)

	for {
		chunk, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}

		if recvErr != nil {
			tmp.Close() //nolint:errcheck

			return status.Errorf(codes.Internal, "upload: receiving data: %v", recvErr)
		}

		if _, err := sink.Write(chunk.GetData()); err != nil {
			tmp.Close() //nolint:errcheck

			return status.Errorf(codes.Internal, "upload: writing: %v", err)
		}
	}

	if err := tmp.Close(); err != nil {
		return status.Errorf(codes.Internal, "upload: closing temp file: %v", err)
	}

	if got := hex.EncodeToString(hasher.Sum(nil)); got != sha {
		return status.Errorf(codes.InvalidArgument, "upload: sha256 mismatch: declared %s, got %s", sha, got)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return status.Errorf(codes.Internal, "upload: committing artifact: %v", err)
	}

	return stream.SendAndClose(&remoteprovisionpb.ArtifactRef{Path: finalPath})
}

// validateSHA256 ensures sha is a 64-char lowercase hex string — both a
// correctness check and a guard against path traversal, since sha is used
// as a cache filename.
func validateSHA256(sha string) error {
	if len(sha) != 64 {
		return errors.New("sha256 must be 64 hex characters")
	}

	for _, c := range sha {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return errors.New("sha256 must be lowercase hex")
		}
	}

	return nil
}
