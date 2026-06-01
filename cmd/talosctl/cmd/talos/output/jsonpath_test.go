// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output_test

import (
	"bytes"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/util/jsonpath"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/output"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

func TestWriteResource(t *testing.T) {
	node := "123.123.123.123"
	event := state.Created

	t.Run("prints scalar values on one line", func(tt *testing.T) {
		var buf bytes.Buffer

		// given
		expectedID := "myCPU"
		processorResource := hardware.NewProcessorInfo(expectedID)
		jsonPath := jsonpath.New("talos")
		assert.Nil(t, jsonPath.Parse("{.metadata.id}"))

		// when
		testObj := output.NewJSONPath(&buf, jsonPath)
		err := testObj.WriteResource(node, processorResource, event)

		// then
		assert.Nil(t, err)

		assert.Equal(t, expectedID+"\n", buf.String())
	})

	t.Run("prints complex values as JSON", func(tt *testing.T) {
		var buf bytes.Buffer

		// given
		expectedMetadata := `{
    "coreCount": 2
}
`
		processorResource := hardware.NewProcessorInfo("myCPU")
		processorResource.TypedSpec().CoreCount = 2
		jsonPath := jsonpath.New("talos")
		assert.Nil(t, jsonPath.Parse("{.spec}"))

		// when
		testObj := output.NewJSONPath(&buf, jsonPath)
		err := testObj.WriteResource(node, processorResource, event)

		// then
		assert.Nil(t, err)

		assert.Equal(t, expectedMetadata, buf.String())
	})
}
