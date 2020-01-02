// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

// GenOption controls generate options specific to input generation.
type GenOption func(o *GenOptions) error

// WithEndpointList specifies endpoints to use when acessing Talos cluster.
func WithEndpointList(endpoints []string) GenOption {
	return func(o *GenOptions) error {
		o.EndpointList = endpoints

		return nil
	}
}

// GenOptions describes generate parameters.
type GenOptions struct {
	EndpointList []string
}

// DefaultGenOptions returns default options.
func DefaultGenOptions() GenOptions {
	return GenOptions{
		EndpointList: []string{"127.0.0.1"},
	}
}
