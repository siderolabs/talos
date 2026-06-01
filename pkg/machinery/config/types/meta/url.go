// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

import "net/url"

// URL wraps the URL with proper YAML marshal/unmarshal.
type URL struct {
	*url.URL
}

// UnmarshalYAML is a custom unmarshaller for `URL`.
func (u *URL) UnmarshalYAML(unmarshal func(any) error) error {
	var endpoint string

	if err := unmarshal(&endpoint); err != nil {
		return err
	}

	if endpoint == "" {
		return nil
	}

	url, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	*u = URL{url}

	return nil
}

// MarshalYAML is a custom marshaller for `URL`.
func (u URL) MarshalYAML() (any, error) {
	if u.URL == nil {
		return "", nil
	}

	return u.URL.String(), nil
}
