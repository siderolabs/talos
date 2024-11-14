// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package registry provides a simple container registry service.
package registry

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
)

// NewService creates a new instance of the registry service.
func NewService(root fs.StatFS, logger *zap.Logger) *Service {
	return &Service{root: root, logger: logger}
}

// Service is a container registry service.
type Service struct {
	logger *zap.Logger
	root   fs.StatFS
}

// Run is an entrypoint to the API service.
func (svc *Service) Run(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v2/{args...}", svc.serveHTTP)

	giveOk := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
	for _, p := range []string{"v2", "healthz"} {
		mux.HandleFunc("GET /"+p, giveOk)
		mux.HandleFunc("GET /"+p+"/{$}", giveOk)
	}

	server := http.Server{Addr: "127.0.0.1:3172", Handler: mux}
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	context.AfterFunc(ctx, func() {
		svc.logger.Info("shutting down registry server", zap.String("addr", server.Addr))

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCtxCancel()

		errCh <- server.Shutdown(shutdownCtx)
	})

	svc.logger.Info("starting registry server", zap.String("addr", server.Addr))

	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}

	cancel()

	err = cmp.Or(err, <-errCh)

	svc.logger.Info("registry server stopped", zap.Error(err))

	return err
}

func (svc *Service) serveHTTP(w http.ResponseWriter, req *http.Request) {
	if err := svc.handler(w, req); err != nil {
		svc.logger.Error("failed to handle request", zap.Error(err))
		w.WriteHeader(getStatusCode(err))
	}
}

func (svc *Service) handler(w http.ResponseWriter, req *http.Request) error {
	logger := svc.logger.With(
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
		zap.String("remote_addr", req.RemoteAddr),
	)

	p, err := extractParams(req)
	if err != nil {
		return fmt.Errorf("failed to extract params: %w", err)
	}

	logger.Info(
		"image request",
		zap.String("name", p.name),
		zap.String("digest", p.dig),
		zap.Bool("is_blob", p.isBlob),
		zap.String("registry", p.registry),
	)

	ref, err := svc.resolveCanonicalRef(p)
	if err != nil {
		return err
	}

	var s content.Store
	if p.isBlob {
		s = &singleFileStore{root: svc.root, path: "blob"}
	} else {
		s = &singleFileStore{root: svc.root, path: filepath.Join("manifests", ref.Name(), "digest")}
	}

	info, err := s.Info(req.Context(), ref.Digest())
	if err != nil {
		return err
	}

	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.Header().Set("Docker-Content-Digest", ref.Digest().String())

	if !p.isBlob {
		manType, manBlob, err := getManifestData(req.Context(), s, ref)
		if err != nil {
			return err
		}

		w.Header().Set("Content-Type", manType)

		if req.Method == http.MethodHead {
			return nil // nothing to do here
		}

		http.ServeContent(w, req, ref.Digest().String(), info.UpdatedAt, bytes.NewReader(manBlob))

		return nil
	}

	reader, err := s.ReaderAt(req.Context(), ocispec.Descriptor{Digest: info.Digest})
	if err != nil {
		return xerrors.NewTaggedf[internalErrorTag]("failed to get content reader: %w", err)
	}

	readerCloser := sync.OnceValue(reader.Close)

	defer readerCloser() //nolint:errcheck

	http.ServeContent(w, req, ref.Digest().String(), info.UpdatedAt, &readSeeker{ReaderAt: reader, Size: info.Size})

	return readerCloser()
}

func (svc *Service) resolveCanonicalRef(p params) (reference.Canonical, error) {
	ref, err := reference.ParseDockerRef(p.String())
	if err != nil {
		return nil, xerrors.NewTaggedf[badRequestTag]("failed to parse docker ref: %w", err)
	}

	cRef, ok := ref.(reference.Canonical)
	if ok {
		return cRef, nil
	}

	namedTagged, ok := ref.(reference.NamedTagged)
	if !ok {
		return nil, xerrors.NewTaggedf[internalErrorTag]("incorrect reference type: %T", ref)
	}

	taggedFile := filepath.Join("manifests", namedTagged.Name(), "reference", namedTagged.Tag())

	ntSum, err := hashFile(taggedFile, svc.root)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, xerrors.NewTaggedf[internalErrorTag]("failed to hash manifest: %w", err)
		}

		return nil, xerrors.NewTagged[notFoundTag](err)
	}

	sha256file := filepath.Join("manifests", namedTagged.Name(), "digest", digest.NewDigestFromBytes(digest.SHA256, ntSum).String())

	sSum, err := hashFile(sha256file, svc.root)
	if err != nil {
		return nil, xerrors.NewTaggedf[internalErrorTag]("failed to hash '%x': %w", sSum, err)
	}

	if !bytes.Equal(ntSum, sSum) {
		return nil, xerrors.NewTaggedf[internalErrorTag]("hash for '%s' is not equal for hash to '%s'", taggedFile, sha256file)
	}

	return &canonical{
		NamedTagged: namedTagged,
		digest:      digest.NewDigestFromBytes(digest.SHA256, ntSum),
	}, nil
}

func hashFile(f string, where fs.FS) (_ []byte, returnErr error) {
	data, err := where.Open(f)
	if err != nil {
		return nil, err
	}

	defer func() { returnErr = cmp.Or(returnErr, data.Close()) }()

	h := sha256.New()
	if _, err = io.Copy(h, data); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func getManifestData(ctx context.Context, store content.Store, ref reference.Canonical) (string, []byte, error) {
	manifestBlob, err := content.ReadBlob(ctx, store, ocispec.Descriptor{Digest: ref.Digest()})
	if err != nil {
		return "", nil, xerrors.NewTaggedf[internalErrorTag]("failed to read content blob: %w", err)
	}

	var manifest struct {
		MediaType string `json:"mediaType"`
	}

	if err = json.Unmarshal(manifestBlob, &manifest); err != nil {
		return "", nil, xerrors.NewTaggedf[internalErrorTag]("failed to unmarshal manifest: %w", err)
	}

	if manifest.MediaType == "" {
		return "", nil, xerrors.NewTaggedf[internalErrorTag]("media type is empty")
	}

	return manifest.MediaType, manifestBlob, nil
}

type canonical struct {
	reference.NamedTagged
	digest digest.Digest
}

func (c *canonical) String() string        { return c.NamedTagged.String() + "@" + c.digest.Encoded() }
func (c *canonical) Digest() digest.Digest { return c.digest }

func getStatusCode(err error) int {
	switch {
	case xerrors.TagIs[notFoundTag](err):
		return http.StatusNotFound
	case xerrors.TagIs[badRequestTag](err):
		return http.StatusBadRequest
	case xerrors.TagIs[internalErrorTag](err):
		fallthrough
	default:
		return http.StatusInternalServerError
	}
}

type (
	notFoundTag      struct{}
	badRequestTag    struct{}
	internalErrorTag struct{}
)
