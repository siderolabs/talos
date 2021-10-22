// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"
	"net/url"

	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/pkg/machinery/config"
)

// Validate checks logging configuration for errors.
func (lc *LoggingConfig) Validate() error {
	var errs *multierror.Error

	for _, dest := range lc.LoggingDestinations {
		var endpoint *url.URL
		if dest.LoggingEndpoint != nil && dest.LoggingEndpoint.URL != nil {
			endpoint = dest.LoggingEndpoint.URL
		}

		if endpoint == nil {
			errs = multierror.Append(errs, fmt.Errorf("empty logging endpoint"))
		} else {
			if endpoint.Host == "" {
				errs = multierror.Append(errs, fmt.Errorf("empty logging endpoint's host"))
			}

			if endpoint.Scheme != "tcp" && endpoint.Scheme != "udp" {
				errs = multierror.Append(errs, fmt.Errorf("unexpected logging endpoint scheme %q", endpoint.Scheme))
			}
		}

		switch f := dest.LoggingFormat; f {
		case "json_lines":
			// nothing
		default:
			errs = multierror.Append(errs, fmt.Errorf("unknown logging format %q", f))
		}
	}

	return errs.ErrorOrNil()
}

// Destinations implements config.Logging interface.
func (lc *LoggingConfig) Destinations() []config.LoggingDestination {
	res := make([]config.LoggingDestination, len(lc.LoggingDestinations))
	for i, ld := range lc.LoggingDestinations {
		res[i] = config.LoggingDestination(ld)
	}

	return res
}

// Endpoint implements config.LoggingDestination interface.
func (ld LoggingDestination) Endpoint() *url.URL {
	return ld.LoggingEndpoint.URL
}

// Format implements config.LoggingDestination interface.
func (ld LoggingDestination) Format() string {
	return ld.LoggingFormat
}
