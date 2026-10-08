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

//go:embed testdata/selinuxpolicy.yaml
var expectedSELinuxPolicyDocument []byte

func TestSELinuxPolicyMarshalStability(t *testing.T) {
	cfg := runtime.NewSELinuxPolicyConfigV1Alpha1("hostmon")
	cfg.PolicyContent = "(type pod_hostmon_t)\n(call pod_domain (pod_hostmon_t))\n"

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedSELinuxPolicyDocument, marshaled)
}

func TestSELinuxPolicyValidate(t *testing.T) {
	t.Parallel()

	_, err := runtime.NewSELinuxPolicyConfigV1Alpha1("").Validate(validationMode{})
	assert.EqualError(t, err, "invalid name \"\": lowercase letters, digits, '-' and '.' only, as a DNS subdomain: domain doesn't match required format: \"\"\ncontent is required")

	cfg := runtime.NewSELinuxPolicyConfigV1Alpha1("hostmon")
	cfg.PolicyContent = "(type pod_hostmon_t)\n"

	_, err = cfg.Validate(validationMode{})
	assert.NoError(t, err)
}
