// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makers

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseOmniAPIUrl validates and parses the omni api url.
func ParseOmniAPIUrl(urlIn string) (*url.URL, error) {
	if !strings.HasPrefix(urlIn, "grpc://") && !strings.HasPrefix(urlIn, "https://") {
		return nil, fmt.Errorf("invalid url scheme: must be either 'grpc://' or 'https://'")
	}

	if !strings.Contains(urlIn, "?jointoken=") {
		return nil, fmt.Errorf("invalid url: must contain a jointoken query parameter")
	}

	url, err := url.Parse(urlIn)
	if err != nil {
		return nil, err
	}

	if url.Port() == "" {
		return nil, fmt.Errorf("invalid url: must contain a port")
	}

	return url, nil
}
