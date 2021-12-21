// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sonobuoy

import "github.com/talos-systems/talos/pkg/version"

type product struct {
	Vendor           string `yaml:"vendor"`
	Name             string `yaml:"name"`
	Version          string `yaml:"version"`
	WebsiteURL       string `yaml:"website_url"`
	RepoURL          string `yaml:"repo_url"`
	DocumentationURL string `yaml:"documentation_url"`
	ProductLogoURL   string `yaml:"product_logo_url"`
	Type             string `yaml:"type"`
	Description      string `yaml:"description"`
}

var talos = product{
	Vendor:           "Talos Systems",
	Name:             "Talos",
	Version:          version.Tag,
	WebsiteURL:       "https://www.siderolabs.com/",
	RepoURL:          "https://github.com/talos-systems/talos",
	DocumentationURL: "https://www.talos.dev",
	ProductLogoURL:   "https://www.talos.dev/images/TalosSystems_Horizontal_Logo_FullColor_RGB-for-site.svg",
	Type:             "installer",
	Description:      "Talos is a modern Kubernetes-focused OS designed to be secure, immutable, and minimal.",
}
