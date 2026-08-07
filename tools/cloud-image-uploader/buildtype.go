// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"regexp"
	"strings"
)

// BuildTypeTagKey is the AWS tag applied to published AMIs and their snapshots so that
// cleanup tooling can tell them apart without parsing names.
const BuildTypeTagKey = "BuildType"

// BuildTypeLabelKey is the GCP equivalent of BuildTypeTagKey. GCP label keys are
// restricted to `[a-z]([-a-z0-9]*[a-z0-9])?`, so the AWS spelling can't be reused.
const BuildTypeLabelKey = "build-type"

// Build types published by this tool.
const (
	// BuildTypeE2E is a throwaway image built for an e2e run.
	BuildTypeE2E = "e2e"
	// BuildTypeNightly is an untagged build off a branch, identified by a git-describe version.
	BuildTypeNightly = "nightly"
	// BuildTypeRelease is a tagged release, including alpha/beta/rc pre-releases.
	BuildTypeRelease = "release"
)

// describeSuffix matches the commits-since-tag and abbreviated SHA that `git describe`
// appends to a version off a branch, e.g. the `-101-g11a7fbe4c` in
// `v1.14.0-alpha.1-101-g11a7fbe4c`.
var describeSuffix = regexp.MustCompile(`-\d+-g[0-9a-f]+$`)

// releaseTag matches a clean release tag, e.g. v1.14.0 or v1.14.0-alpha.1.
var releaseTag = regexp.MustCompile(`^v\d+\.\d+\.\d+(-(alpha|beta|rc)\.\d+)?$`)

// DeriveBuildType works out the build type of the images being published.
//
// An e2e run names its images with a prefix. Everything else is versioned by
// `git describe`, which appends a commit count and SHA when HEAD isn't exactly on a
// tag -- that suffix is what separates a nightly from a release.
func DeriveBuildType(tag, namePrefix string) string {
	if strings.Contains(namePrefix, "e2e") {
		return BuildTypeE2E
	}

	if describeSuffix.MatchString(tag) {
		return BuildTypeNightly
	}

	if releaseTag.MatchString(tag) {
		return BuildTypeRelease
	}

	// not a recognizable release tag, so treat it as a branch build and let it age out
	// rather than accumulating forever.
	return BuildTypeNightly
}
