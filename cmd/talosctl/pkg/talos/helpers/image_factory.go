// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"net/url"
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
