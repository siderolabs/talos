// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "net/url"

// RuntimeConfig defines the interface to access Talos runtime configuration.
type RuntimeConfig interface {
	EventsEndpoint() *string
	KmsgLogURLs() []*url.URL
}

// WrapRuntimeConfigList wraps a list of RuntimeConfig into a single RuntimeConfig aggregating the results.
func WrapRuntimeConfigList(configs ...RuntimeConfig) RuntimeConfig {
	return runtimeConfigWrapper(configs)
}

type runtimeConfigWrapper []RuntimeConfig

func (w runtimeConfigWrapper) EventsEndpoint() *string {
	return findFirstValue(w, func(c RuntimeConfig) *string {
		return c.EventsEndpoint()
	})
}

func (w runtimeConfigWrapper) KmsgLogURLs() []*url.URL {
	return aggregateValues(w, func(c RuntimeConfig) []*url.URL {
		return c.KmsgLogURLs()
	})
}

func findFirstValue[T any, R any](documents []T, getter func(T) *R) *R {
	for _, document := range documents {
		if value := getter(document); value != nil {
			return value
		}
	}

	return nil
}

func aggregateValues[T any, R any](documents []T, getter func(T) []R) []R {
	var result []R

	for _, document := range documents {
		result = append(result, getter(document)...)
	}

	return result
}
