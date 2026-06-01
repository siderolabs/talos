// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8sjson provides a bridge and helpers for working with unstructured/json/yaml Kubernetes objects.
package k8sjson

// DeepCopyToJSON deep-copies a YAML-decoded value into one that uses only
// JSON-compatible types (string, bool, int64, float64, nil, []any,
// map[string]any).
//
// The YAML parser emits Go int / uint values for integers,
// but k8s.io/apimachinery's runtime.DeepCopyJSON and unstructured helpers
// panic on those, so any int-shaped field (e.g. topologySpread maxSkew) would
// crash the controller. Unknown types are returned as-is so the typed schema
// validation upstream is what surfaces invalid input, not this helper.
//
//nolint:gocyclo
func DeepCopyToJSON(x any) any {
	switch x := x.(type) {
	case map[string]any:
		if x == nil {
			return x
		}

		clone := make(map[string]any, len(x))
		for k, v := range x {
			clone[k] = DeepCopyToJSON(v)
		}

		return clone
	case []any:
		if x == nil {
			return x
		}

		clone := make([]any, len(x))
		for i, v := range x {
			clone[i] = DeepCopyToJSON(v)
		}

		return clone
	case int:
		return int64(x)
	case int8:
		return int64(x)
	case int16:
		return int64(x)
	case int32:
		return int64(x)
	case uint:
		return int64(x)
	case uint8:
		return int64(x)
	case uint16:
		return int64(x)
	case uint32:
		return int64(x)
	case uint64:
		return float64(x)
	case float32:
		return float64(x)
	default:
		return x
	}
}
