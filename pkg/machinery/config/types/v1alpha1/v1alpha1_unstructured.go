// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"
	"slices"
)

// Unstructured allows wrapping any map[string]interface{} into a config object.
//
// docgen: nodoc
// +k8s:deepcopy-gen=true
type Unstructured struct {
	Object map[string]any `yaml:",inline"`
}

// DeepCopy performs copying of the Object contents.
func (in *Unstructured) DeepCopy() *Unstructured {
	if in == nil {
		return nil
	}

	out := new(Unstructured)

	out.Object = deepCopyUnstructured(in.Object).(map[string]any) //nolint:errcheck,forcetypeassert

	return out
}

func deepCopyUnstructured(x any) any {
	switch x := x.(type) {
	case map[string]any:
		if x == nil {
			return x
		}

		clone := make(map[string]any, len(x))

		for k, v := range x {
			clone[k] = deepCopyUnstructured(v)
		}

		return clone
	case []any:
		if x == nil {
			return x
		}

		clone := make([]any, len(x))

		for i, v := range x {
			clone[i] = deepCopyUnstructured(v)
		}

		return clone
	case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, complex64, complex128, nil:
		return x
	case []byte:
		return slices.Clone(x)
	default:
		panic(fmt.Errorf("cannot deep copy %T", x))
	}
}
