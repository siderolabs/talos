// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseImageFactoryURL parses an image factory url and errors if scheme or host is missing.
func ParseImageFactoryURL(urlStr string) (*url.URL, error) {
	factoryURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("malformed Image Factory URL: %q: %w", urlStr, err)
	}

	if factoryURL.Scheme == "" || factoryURL.Host == "" {
		return nil, fmt.Errorf("image Factory URL must include scheme and host: %q", urlStr)
	}

	return factoryURL, nil
}

// BuildInstallerImageRef builds an installer image reference of the form
// <factory host>/<platform>-installer[-secureboot]/<schematic ID>:<version>.
func BuildInstallerImageRef(factoryHost, schematic, version, platform string, secureBoot bool) string {
	installerType := platform + "-installer"
	if secureBoot {
		installerType += "-secureboot"
	}

	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	return fmt.Sprintf("%s/%s/%s:%s", factoryHost, installerType, schematic, version)
}
