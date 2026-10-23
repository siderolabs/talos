// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package redactable_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/redactable"
)

type testSpec struct {
	Secret string
}

func (spec testSpec) DeepCopy() testSpec { return spec }

func (spec *testSpec) HasSecrets(resource.Metadata) bool { return spec.Secret != "" }

func (spec *testSpec) RedactSecrets(resource.Metadata) { spec.Secret = "<REDACTED>" }

type sensitiveExtension struct{}

func (sensitiveExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:        "SensitiveTests.test.talos.dev",
		Sensitivity: meta.Sensitive,
	}
}

type nonSensitiveExtension struct{}

func (nonSensitiveExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: "NonSensitiveTests.test.talos.dev",
	}
}

func TestRegister(t *testing.T) {
	t.Parallel()

	redactable.Register[testSpec, sensitiveExtension]()

	assert.True(t, redactable.IsRedactable("SensitiveTests.test.talos.dev"))
}

func TestRegisterNonSensitive(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(
		t,
		`resource type "NonSensitiveTests.test.talos.dev" is registered as redactable, but it is not sensitive`,
		redactable.Register[testSpec, nonSensitiveExtension],
	)

	assert.False(t, redactable.IsRedactable("NonSensitiveTests.test.talos.dev"))
}
