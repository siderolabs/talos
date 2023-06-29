// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package endpoint has common tools for parsing http API endpoints.
package endpoint

import (
	"net/url"
	"regexp"
)

var urlSchemeMatcher = regexp.MustCompile(`[a-zA-z]+://`)

// Endpoint defines all params parsed from the API endpoint.
type Endpoint struct {
	Host     string
	Insecure bool
	params   url.Values
}

// Parse parses the endpoint from string.
func Parse(sideroLinkParam string) (Endpoint, error) {
	if !urlSchemeMatcher.MatchString(sideroLinkParam) {
		sideroLinkParam = "grpc://" + sideroLinkParam
	}

	u, err := url.Parse(sideroLinkParam)
	if err != nil {
		return Endpoint{}, err
	}

	result := Endpoint{
		Host:     u.Host,
		Insecure: u.Scheme == "grpc",
		params:   u.Query(),
	}

	if u.Port() == "" && u.Scheme == "https" {
		result.Host += ":443"
	}

	return result, nil
}

// GetParam reads param from the query.
func (e *Endpoint) GetParam(name string) string {
	return e.params.Get(name)
}
