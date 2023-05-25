// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
)

func TestRedact(t *testing.T) {
	cfg := siderolink.NewConfigV1Alpha1()
	cfg.APIUrlConfig.URL = must(url.Parse("https://siderolink.api/join?jointoken=secret&user=alice"))

	assert.Equal(t, "https://siderolink.api/join?jointoken=secret&user=alice", cfg.SideroLink().APIUrl().String())

	cfg.Redact("REDACTED")

	assert.Equal(t, "https://siderolink.api/join?jointoken=REDACTED&user=alice", cfg.APIUrlConfig.String())
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}
