// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package imageref provides canonical normalization of container image references.
//
// Talos parses an image reference in several places on a single pull: to match it against the
// image verification policy, to look up the registry mirror, auth and TLS configuration, and to
// hand it to containerd for the actual pull. All of those have to agree on a single spelling of
// the reference, otherwise the policy decision can be made about one image while a different one
// is pulled.
package imageref

import (
	"fmt"
	"strings"

	"github.com/distribution/reference"
)

const (
	// dockerHubDomain is the canonical Docker Hub domain as produced by
	// github.com/distribution/reference and understood by containerd.
	dockerHubDomain = "docker.io"

	// legacyDockerHubDomain is the legacy Docker Hub domain. It is also the spelling
	// github.com/google/go-containerregistry canonicalizes Docker Hub to, so references and
	// patterns do show up written this way.
	legacyDockerHubDomain = "index.docker.io"

	// endpointDockerHubDomain is the Docker Hub registry endpoint, the host `docker.io` is an
	// alias for (see containerd's docker.DefaultHost). A reference written this way points at
	// the very same image as one written against `docker.io`.
	endpointDockerHubDomain = "registry-1.docker.io"

	// officialRepoPrefix is the implicit namespace of the official Docker Hub images.
	officialRepoPrefix = "library/"

	// localhostDomain is always a registry domain, never a repository namespace.
	localhostDomain = "localhost"
)

// Parse parses a container image reference and returns it in the canonical form used throughout
// Talos.
//
// On top of the normalization done by github.com/distribution/reference (defaulting the domain,
// adding the implicit `library/` namespace on Docker Hub, defaulting the tag to `latest` and
// dropping the tag from a reference carrying both a tag and a digest), the registry domain is
// lower-cased, as registry domains are DNS names and DNS is case-insensitive, and the alternative
// spellings of the Docker Hub domain, `index.docker.io` and `registry-1.docker.io`, are folded
// into `docker.io`.
//
// Normalization is idempotent: parsing the result again yields the same reference.
func Parse(imageRef string) (reference.Named, error) {
	namedRef, err := reference.ParseDockerRef(imageRef)
	if err != nil {
		return nil, err
	}

	domain := reference.Domain(namedRef)

	normalizedDomain := normalizeDomain(domain)
	if normalizedDomain == domain {
		return namedRef, nil
	}

	if !isExplicitDomain(normalizedDomain) {
		// A single-label component is taken for a registry domain only because it is not
		// lower-case: lower-casing it would turn the registry into a Docker Hub namespace and
		// point the reference at a completely different image, so reject it instead.
		return nil, fmt.Errorf("invalid reference format: registry domain %q must be lowercase", domain)
	}

	path := reference.Path(namedRef)

	// the implicit `library/` namespace applies once the domain is folded into `docker.io`,
	// e.g. `INDEX.DOCKER.IO/alpine` is `docker.io/library/alpine`
	if normalizedDomain == dockerHubDomain && !strings.ContainsRune(path, '/') {
		path = officialRepoPrefix + path
	}

	// build the reference explicitly rather than re-parsing the normalized string: re-parsing
	// would re-run the domain/repository heuristics, which are case-sensitive
	trimmedRef, err := reference.WithName(normalizedDomain + "/" + path)
	if err != nil {
		return nil, err
	}

	switch ref := namedRef.(type) {
	case reference.Canonical:
		return reference.WithDigest(trimmedRef, ref.Digest())
	case reference.NamedTagged:
		return reference.WithTag(trimmedRef, ref.Tag())
	default:
		return trimmedRef, nil
	}
}

// RepositoryKey returns the canonical `<registry>/<repository>` form of the reference, with the
// tag and the digest stripped.
//
// This is the key image verification rules are matched against.
func RepositoryKey(namedRef reference.Named) string {
	return reference.TrimNamed(namedRef).String()
}

// NormalizePattern normalizes the registry domain of an image reference glob pattern, so that the
// pattern is matched in the same namespace the output of [RepositoryKey] is in.
//
// Only the domain part of the pattern is normalized: the repository part of a canonical reference
// is always lower-case already, so folding its case would make a pattern match references it was
// not written for.
func NormalizePattern(pattern string) string {
	domain, repository, hasRepository := strings.Cut(pattern, "/")

	// a pattern which carries no `/` yet is still a pattern on the domain: the domain is the
	// first thing the normalized reference starts with, so `index.docker.io*` has to be folded
	// just like `index.docker.io/*` is
	normalizedDomain := normalizeDomainPattern(domain)

	if !hasRepository {
		return normalizedDomain
	}

	return normalizedDomain + "/" + repository
}

// normalizeDomainPattern normalizes the glob pattern matched against a registry domain.
func normalizeDomainPattern(domain string) string {
	globIdx := strings.IndexRune(domain, '*')
	if globIdx < 0 {
		return normalizeDomain(domain)
	}

	// the glob matcher anchors the literal in front of the first `*` at the start of the
	// reference, so that literal is normalized, while whatever the glob itself covers is matched
	// as written
	return normalizeDomain(domain[:globIdx]) + domain[globIdx:]
}

// normalizeDomain normalizes a registry domain to its canonical spelling.
func normalizeDomain(domain string) string {
	domain = strings.ToLower(domain)

	switch domain {
	case legacyDockerHubDomain, endpointDockerHubDomain:
		return dockerHubDomain
	default:
		return domain
	}
}

// isExplicitDomain reports whether the component is recognized as a registry domain by
// github.com/distribution/reference independently of its case.
func isExplicitDomain(domain string) bool {
	return strings.ContainsAny(domain, ".:") || domain == localhostDomain
}
