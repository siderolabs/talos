// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

// Package k8s provides Kubernetes integration tests.
package k8s

const (
	// RookCephHelmChartVersion is the version of the Rook Ceph Helm chart to use.
	// renovate: datasource=helm versioning=helm depName=rook-ceph registryUrl=https://charts.rook.io/release
	RookCephHelmChartVersion = "v1.17.6"
	// LongHornHelmChartVersion is the version of the Longhorn Helm chart to use.
	// renovate: datasource=helm versioning=helm depName=longhorn registryUrl=https://charts.longhorn.io
	LongHornHelmChartVersion = "v1.9.0"
	// OpenEBSChartVersion is the version of the OpenEBS Helm chart to use.
	// renovate: datasource=helm versioning=helm depName=openebs registryUrl=https://openebs.github.io/openebs
	OpenEBSChartVersion = "v4.2.0"
)
