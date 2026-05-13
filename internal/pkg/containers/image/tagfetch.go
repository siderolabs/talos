// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/errdefs"
	"github.com/google/go-containerregistry/pkg/name"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/siderolabs/talos/internal/pkg/containers/image/verify"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// maxTagManifestSize bounds the response body read by the tag-based manifest fallback.
const maxTagManifestSize = 4 * 1024 * 1024

// tagFetchAccept is the Accept header sent on tag-based manifest requests, matching
// the resolver's resolveHeader so the registry returns the same media types.
var tagFetchAccept = strings.Join([]string{
	ocispec.MediaTypeImageManifest,
	ocispec.MediaTypeImageIndex,
	"application/vnd.docker.distribution.manifest.v2+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"*/*",
}, ", ")

// NewTagFetcher builds a verify.TagFetcher that fetches a manifest by its tag URL
// through the same RegistryHosts (auth + TLS + mirror list) used by the resolver.
//
// It is the fallback used by signature verification when the resolver's
// digest-based manifest fetch returns NotFound, e.g. against registry.k8s.io's
// CDN where the HEAD-by-tag and GET-by-digest requests can be routed to
// different regional backends with inconsistent replication.
func NewTagFetcher(reg cri.Registries) verify.TagFetcher {
	hosts := RegistryHosts(reg)

	return func(ctx context.Context, repo name.Repository, tag string, expectedDigest digest.Digest) ([]byte, error) {
		registryHosts, err := hosts(repo.RegistryStr())
		if err != nil {
			return nil, fmt.Errorf("failed to get registry hosts for %q: %w", repo.RegistryStr(), err)
		}

		if len(registryHosts) == 0 {
			return nil, fmt.Errorf("no registry hosts for %q: %w", repo.RegistryStr(), errdefs.ErrNotFound)
		}

		var firstErr error

		for _, h := range registryHosts {
			body, err := fetchManifestByTagFromHost(ctx, h, repo, tag, expectedDigest)
			if err == nil {
				return body, nil
			}

			// Prefer a non-NotFound error over a NotFound one, so the caller surfaces the
			// most actionable diagnostic (auth/TLS/network) rather than the generic 404.
			if firstErr == nil || (errdefs.IsNotFound(firstErr) && !errdefs.IsNotFound(err)) {
				firstErr = err
			}
		}

		return nil, firstErr
	}
}

// fetchManifestByTagFromHost issues a GET against host's tag URL for the
// manifest, performs the same 401/Authorize handshake the docker resolver does,
// and validates the returned content against expectedDigest.
func fetchManifestByTagFromHost(
	ctx context.Context, host docker.RegistryHost, repo name.Repository, tag string, expectedDigest digest.Digest,
) ([]byte, error) {
	reqURL := buildTagManifestURL(host, repo, tag)

	const maxAuthRetries = 5

	// Authorizer.AddResponses inspects the trailing responses to detect repeated
	// challenges from the same request (see containerd's invalidAuthorization),
	// so we accumulate the slice across retries rather than passing only the
	// latest response.
	var responses []*http.Response

	for range maxAuthRetries {
		//nolint:bodyclose // body is read and closed inside doTagManifestRequest; the returned *http.Response is a body-less snapshot for Authorizer.AddResponses
		body, status, header, snap, err := doTagManifestRequest(ctx, host, reqURL)
		if err != nil {
			return nil, err
		}

		if status == http.StatusUnauthorized && host.Authorizer != nil {
			responses = append(responses, snap) //nolint:bodyclose // snap has no body

			retry, authErr := tryRefreshAuthorization(ctx, host.Authorizer, responses)
			if authErr != nil {
				return nil, authErr
			}

			if retry {
				continue
			}

			return nil, fmt.Errorf("unauthorized fetching %s", reqURL)
		}

		if err := checkTagManifestStatus(status, reqURL); err != nil {
			return nil, err
		}

		if err := verifyTagManifestDigest(body, header.Get("Docker-Content-Digest"), expectedDigest); err != nil {
			return nil, err
		}

		return body, nil
	}

	return nil, errors.New("too many authorization retries")
}

// doTagManifestRequest sends a single GET to the manifest tag URL. It returns the
// decoded body bytes plus only the response fields the caller needs (status,
// headers, and a body-less snapshot suitable for [docker.Authorizer.AddResponses]).
// The response body is fully read and closed before returning, so the caller has
// no body lifecycle to manage.
func doTagManifestRequest(
	ctx context.Context, host docker.RegistryHost, reqURL string,
) ([]byte, int, http.Header, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, 0, nil, nil, fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Accept", tagFetchAccept)

	if host.Authorizer != nil {
		if err := host.Authorizer.Authorize(ctx, req); err != nil {
			return nil, 0, nil, nil, fmt.Errorf("failed to authorize: %w", err)
		}
	}

	client := host.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, nil, nil, fmt.Errorf("failed to GET %s: %w", reqURL, err)
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxTagManifestSize))

	if closeErr := resp.Body.Close(); closeErr != nil && readErr == nil {
		readErr = closeErr
	}

	if readErr != nil {
		return nil, 0, nil, nil, fmt.Errorf("failed to read manifest body: %w", readErr)
	}

	// Authorizer.AddResponses only inspects status/headers, not the body, so a snapshot
	// without the body is sufficient and avoids keeping the connection open.
	snap := &http.Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Request:    resp.Request,
	}

	return body, resp.StatusCode, resp.Header, snap, nil
}

// tryRefreshAuthorization invokes the authorizer's response handler so it can
// extract bearer challenges. Returns true when the caller should retry.
func tryRefreshAuthorization(ctx context.Context, authorizer docker.Authorizer, responses []*http.Response) (bool, error) {
	addErr := authorizer.AddResponses(ctx, responses)
	if addErr == nil {
		return true, nil
	}

	if errdefs.IsNotImplemented(addErr) {
		return false, nil
	}

	return false, fmt.Errorf("failed to refresh authorization: %w", addErr)
}

func checkTagManifestStatus(status int, reqURL string) error {
	if status == http.StatusNotFound {
		return fmt.Errorf("manifest at %s: %w", reqURL, errdefs.ErrNotFound)
	}

	if status >= 300 {
		return fmt.Errorf("unexpected status %d fetching %s", status, reqURL)
	}

	return nil
}

// buildTagManifestURL mirrors how containerd's dockerBase builds a manifests URL:
// <scheme>://<host>/<host.Path>/<repo>/manifests/<tag>, with the proxy-namespace
// query argument added when the configured host is not the image's registry.
func buildTagManifestURL(host docker.RegistryHost, repo name.Repository, tag string) string {
	p := path.Join("/", host.Path, repo.RepositoryStr(), "manifests", tag)

	u := url.URL{
		Scheme: host.Scheme,
		Host:   host.Host,
		Path:   p,
	}

	// Proxy hosts (mirror endpoints whose Host differs from the image's registry) expect
	// the upstream namespace as ?ns=<registry> so they know what to proxy.
	// docker.DefaultHost handles aliases such as docker.io → registry-1.docker.io,
	// so we don't have to duplicate that mapping here.
	refHost := repo.RegistryStr()

	canonicalRefHost, err := docker.DefaultHost(refHost)
	if err != nil {
		canonicalRefHost = refHost
	}

	if canonicalRefHost != host.Host {
		q := u.Query()
		q.Set("ns", refHost)
		u.RawQuery = q.Encode()
	}

	return u.String()
}

func verifyTagManifestDigest(body []byte, serverDigestHeader string, expectedDigest digest.Digest) error {
	// Compute the body digest using the same algorithm the expected digest uses,
	// rather than hard-coding sha256 via digest.FromBytes: OCI permits other
	// algorithms (e.g. sha512) and the comparison must track whatever the
	// resolver advertised.
	algo := expectedDigest.Algorithm()
	if !algo.Available() {
		return fmt.Errorf("tag fetch digest algorithm %q is not available", algo)
	}

	actualDigest := algo.FromBytes(body)
	if actualDigest != expectedDigest {
		return fmt.Errorf("tag fetch digest mismatch: got %s, expected %s", actualDigest, expectedDigest)
	}

	if serverDigestHeader != "" {
		serverDigest := digest.Digest(serverDigestHeader)
		if serverDigest != expectedDigest {
			return fmt.Errorf("tag fetch server digest mismatch: header %s, expected %s", serverDigest, expectedDigest)
		}
	}

	return nil
}
