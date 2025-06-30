// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"
	"strconv"
)

// CPURequests implements the config.Resources interface.
func (r *ResourcesConfig) CPURequests() string {
	if r != nil {
		return convertResource(r.Requests, "cpu")
	}

	return ""
}

// MemoryRequests implements the config.Resources interface.
func (r *ResourcesConfig) MemoryRequests() string {
	if r != nil {
		return convertResource(r.Requests, "memory")
	}

	return ""
}

// CPULimits implements the config.Resources interface.
func (r *ResourcesConfig) CPULimits() string {
	if r != nil {
		return convertResource(r.Limits, "cpu")
	}

	return ""
}

// MemoryLimits implements the config.Resources interface.
func (r *ResourcesConfig) MemoryLimits() string {
	if r != nil {
		return convertResource(r.Limits, "memory")
	}

	return ""
}

// Validate performs config validation.
func (r *ResourcesConfig) Validate() error {
	if r == nil {
		return nil
	}

	checkKeys := func(resource Unstructured) error {
		for key := range resource.Object {
			switch key {
			case "memory":
			case "cpu":
			default:
				return fmt.Errorf("unsupported pod resource %q", key)
			}
		}

		return nil
	}
	if err := checkKeys(r.Requests); err != nil {
		return err
	}

	return checkKeys(r.Limits)
}

func convertResource(resources Unstructured, key string) string {
	if resources.Object == nil {
		return ""
	}

	if _, ok := resources.Object[key]; !ok {
		return ""
	}

	val := resources.Object[key]
	switch typedVal := val.(type) {
	case int:
		return strconv.Itoa(typedVal)
	case string:
		return typedVal
	default:
		return ""
	}
}
