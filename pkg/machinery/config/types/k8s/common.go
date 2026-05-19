// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"fmt"
	"strconv"

	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

// ResourcesConfig represents the pod resources.
type ResourcesConfig struct {
	//   description: |
	//     Requests configures the reserved cpu/memory resources.
	//   examples:
	//     - name: resources requests.
	//       value: resourcesConfigRequestsExample()
	//   schema:
	//     type: object
	Requests meta.Unstructured `yaml:"requests,omitempty"`
	//   description: |
	//     Limits configures the maximum cpu/memory limits a pod can use.
	//   examples:
	//     - name: resources limits.
	//       value: resourcesConfigLimitsExample()
	//   schema:
	//     type: object
	Limits meta.Unstructured `yaml:"limits,omitempty"`
}

func resourcesConfigRequestsExample() meta.Unstructured {
	return meta.Unstructured{
		Object: map[string]any{
			"cpu":    1,
			"memory": "1Gi",
		},
	}
}

func resourcesConfigLimitsExample() meta.Unstructured {
	return meta.Unstructured{
		Object: map[string]any{
			"cpu":    2,
			"memory": "2500Mi",
		},
	}
}

// CPURequests implements the config.Resources interface.
func (r ResourcesConfig) CPURequests() string {
	return convertResource(r.Requests, "cpu")
}

// MemoryRequests implements the config.Resources interface.
func (r ResourcesConfig) MemoryRequests() string {
	return convertResource(r.Requests, "memory")
}

// CPULimits implements the config.Resources interface.
func (r ResourcesConfig) CPULimits() string {
	return convertResource(r.Limits, "cpu")
}

// MemoryLimits implements the config.Resources interface.
func (r ResourcesConfig) MemoryLimits() string {
	return convertResource(r.Limits, "memory")
}

// Validate performs config validation.
func (r ResourcesConfig) Validate() error {
	checkKeys := func(resource meta.Unstructured) error {
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

func convertResource(resources meta.Unstructured, key string) string {
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
