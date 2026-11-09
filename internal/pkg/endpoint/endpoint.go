// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package endpoint has common tools for parsing http API endpoints.
package endpoint

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var urlSchemeMatcher = regexp.MustCompile(`[a-zA-z]+://`)

// redactedValue is the replacement for sensitive values in the redacted endpoint.
const redactedValue = "redacted"

// sensitiveParams is the list of query parameters which carry secrets.
var sensitiveParams = []string{"jointoken"}

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
		// url.Error includes the full URL, which might contain the join token, so strip it
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			return Endpoint{}, fmt.Errorf("failed to parse endpoint %q: %w", Redact(sideroLinkParam), urlErr.Err)
		}

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

// Redact returns the endpoint with sensitive query parameters (join token) redacted.
//
// The result is suitable for logging.
func Redact(sideroLinkParam string) string {
	base, rawQuery, found := strings.Cut(sideroLinkParam, "?")
	if !found {
		return sideroLinkParam
	}

	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		// failed to parse the query, so redact it completely
		return base + "?" + redactedValue
	}

	for _, param := range sensitiveParams {
		if query.Has(param) {
			query.Set(param, redactedValue)
		}
	}

	return base + "?" + query.Encode()
}
