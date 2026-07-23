// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
)

//go:embed testdata/udevrules.yaml
var expectedUdevRulesDocument []byte

func TestUdevRulesMarshalStability(t *testing.T) {
	cfg := runtime.NewUdevRulesConfigV1Alpha1()
	cfg.UdevRules = []string{`SUBSYSTEM=="drm", KERNEL=="renderD*", GROUP="44", MODE="0660"`}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedUdevRulesDocument, marshaled)
}

func TestUdevRules(t *testing.T) {
	cfg := runtime.NewUdevRulesConfigV1Alpha1()
	cfg.UdevRules = []string{"rule1", "rule2"}

	assert.Equal(t, []string{"rule1", "rule2"}, cfg.Rules())
}
