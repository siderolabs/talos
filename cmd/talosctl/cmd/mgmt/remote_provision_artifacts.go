// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package mgmt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
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
