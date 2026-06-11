// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	"slices"
	"strings"

	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	v1 "k8s.io/api/core/v1"
)

// EnvVars produces Kubernetes env vars spec from a map.
func EnvVars(environment map[string]string) []v1.EnvVar {
	if len(environment) == 0 {
		return nil
	}

	keys := maps.Keys(environment)
	slices.Sort(keys)

	return xslices.Map(keys, func(key string) v1.EnvVar {
		// Kubernetes supports variable references in variable values, so escape '$' to prevent that.
		return v1.EnvVar{
			Name:  key,
			Value: strings.ReplaceAll(environment[key], "$", "$$"),
		}
	})
}
