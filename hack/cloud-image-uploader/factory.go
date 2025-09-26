// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"golang.org/x/sync/errgroup"
)

var extensions = map[string]string{
	"aws": ".raw.xz",
	"gcp": ".raw.tar.gz",
}

// FactoryDownloader is helper for downloading images from Image Factory.
type FactoryDownloader struct {
	Target  string
	Options Options
}

func (f *FactoryDownloader) Download(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, arch := range f.Options.Architectures {
		g.Go(func() error {
			artifact := fmt.Sprintf("%s-%s%s", f.Target, arch, extensions[f.Target])

			r, err := f.getArtifact(ctx, artifact)
			if err != nil {
				return err
			}
			defer r.Close() //nolint:errcheck

			return f.saveArtifact(artifact, r)
		})
	}

	return g.Wait()
}

func (f *FactoryDownloader) getArtifact(ctx context.Context, name string) (io.ReadCloser, error) {
	url, err := url.JoinPath(
		f.Options.FactoryHost,
		"images",
		f.Options.SchematicFor(f.Target),
		f.Options.Tag,
		name,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image from %q: %w", url, err)
	}

	return resp.Body, nil
}

func (f *FactoryDownloader) saveArtifact(name string, r io.Reader) error {
	artifact := filepath.Join(f.Options.ArtifactsPath, name)

	of, err := os.OpenFile(artifact, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", artifact, err)
	}
	defer of.Close() //nolint:errcheck

	_, err = io.Copy(of, r)
	if err != nil {
		return fmt.Errorf("failed to write image to file %q: %w", artifact, err)
	}

	return nil
}
