// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sonobuoy

import "github.com/siderolabs/talos/pkg/machinery/version"

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
	ContactEmail     string `yaml:"contact_email_address"`
}

var talos = product{
	Vendor:           "Sidero Labs",
	Name:             "Talos Linux",
	Version:          version.Tag,
	WebsiteURL:       "https://www.siderolabs.com/",
	RepoURL:          "https://github.com/siderolabs/talos",
	DocumentationURL: "https://www.talos.dev",
	ProductLogoURL:   "https://www.talos.dev/images/Sidero_stacked_darkbkgd_RGB.svg",
	Type:             "installer",
	Description:      "Talos Linux is Linux designed for Kubernetes - secure, immutable, and minimal.",
	ContactEmail:     "developers@siderolabs.com",
}
