// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging_test

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/logging"
)

func TestCircularBufferRoundTrip(t *testing.T) {
	t.Parallel()

	manager := logging.NewCircularBufferLoggingManager(log.New(io.Discard, "", 0))

	handler := manager.ServiceLog("test")

	w, err := handler.Writer()
	require.NoError(t, err)

	// write enough to rotate (compress) multiple chunks, but less than the buffer capacity
	var expected bytes.Buffer

	for i := 0; expected.Len() < 8*logging.ChunkCapacity+logging.ChunkCapacity/2; i++ {
		line := fmt.Sprintf("%d level=info msg=\"request handled\" path=/v1/items/%d duration=%dms status=%d\n", 1700000000+i, i*7919%10007, i%997, 200+i%3)

		expected.WriteString(line)

		_, err = w.Write([]byte(line))
		require.NoError(t, err)
	}

	r, err := handler.Reader()
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, r.Close()) })

	actual, err := io.ReadAll(r)
	require.NoError(t, err)

	require.Equal(t, expected.String(), string(actual))
}
