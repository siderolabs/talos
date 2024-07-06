// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

func TestUnstructuredDeepCopy(t *testing.T) {
	u := v1alpha1.Unstructured{
		Object: map[string]any{
			"strings": map[string]any{
				"foo": "bar",
			},
			"numbers": []any{
				map[string]any{
					"int":    32,
					"int8":   int8(34),
					"byte":   byte(35),
					"int16":  int16(36),
					"int32":  int32(37),
					"int64":  int64(38),
					"uint":   uint(39),
					"uint8":  uint8(40),
					"uint16": uint16(41),
					"uint32": uint32(42),
					"uint64": uint64(43),
				},
				float32(44.0),
				float64(45.0),
				complex64(complex(46.0, 47.0)),
				complex128(complex(48.0, 49.0)),
			},
			"bytes": []byte("abc"),
		},
	}

	assert.Equal(t, u.DeepCopy(), &u)
}
