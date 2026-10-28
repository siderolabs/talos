// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// TestMetaKeyRedactSecrets covers the part which redactgen can't express: whether a MetaKey value is
// a secret follows from the resource ID, so the same spec is redacted under one key and left alone
// under another.
func TestMetaKeyRedactSecrets(t *testing.T) {
	t.Parallel()

	const value = "not-for-your-eyes"

	for _, test := range []struct {
		name     string
		tag      uint8
		redacted bool
	}{
		{
			name:     "node unique token",
			tag:      meta.UniqueMachineToken,
			redacted: true,
		},
		{
			// the value is redacted as a whole here: the redactor which takes out just the secrets is
			// registered by the network resource package, which this package can't import, see
			// TestMetalNetworkPlatformConfigRedaction over there
			name:     "metal platform network config",
			tag:      meta.MetalNetworkPlatformConfig,
			redacted: true,
		},
		{
			name:     "state encryption config",
			tag:      meta.StateEncryptionConfig,
			redacted: true,
		},
		{
			name: "upgrade marker",
			tag:  meta.Upgrade,
		},
		{
			name: "download URL code",
			tag:  meta.DownloadURLCode,
		},
		{
			name: "user reserved",
			tag:  meta.UserReserved1,
		},
		{
			name: "UUID override",
			tag:  meta.UUIDOverride,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			key := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(test.tag))
			key.TypedSpec().Value = value

			md := *key.Metadata()
			spec := key.TypedSpec()

			assert.Equal(t, test.redacted, spec.HasSecrets(md))

			spec.RedactSecrets(md)

			if test.redacted {
				assert.Equal(t, constants.Redacted, spec.Value)
			} else {
				assert.Equal(t, value, spec.Value)
			}
		})
	}
}

// TestMetaKeyRedactEmpty makes sure an unset key is never reported as carrying a secret: the
// annotation the API adds is supposed to mean that something was actually taken away.
func TestMetaKeyRedactEmpty(t *testing.T) {
	t.Parallel()

	key := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.UniqueMachineToken))

	assert.False(t, key.TypedSpec().HasSecrets(*key.Metadata()))
}

// TestMetaKeyRedactUnknownID guards the lookup against IDs which are not META tags at all.
func TestMetaKeyRedactUnknownID(t *testing.T) {
	t.Parallel()

	key := runtime.NewMetaKey(runtime.NamespaceName, "not-a-meta-tag")
	key.TypedSpec().Value = "whatever"

	assert.False(t, key.TypedSpec().HasSecrets(*key.Metadata()))
}

// TestMetaKeyRedactIsDeepCopySafe documents the contract the API relies on: redaction mutates in
// place, so the caller has to copy first, and the copy must not reach back into the original.
func TestMetaKeyRedactIsDeepCopySafe(t *testing.T) {
	t.Parallel()

	key := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.StateEncryptionConfig))
	key.TypedSpec().Value = "secret"

	redacted, ok := key.DeepCopy().(*runtime.MetaKey)
	assert.True(t, ok)

	redacted.TypedSpec().RedactSecrets(*redacted.Metadata())

	assert.Equal(t, constants.Redacted, redacted.TypedSpec().Value)
	assert.Equal(t, "secret", key.TypedSpec().Value)
}

// TestRegisterMetaKeyValueRedactorUnknownKey makes sure a redactor can only ever refine the redaction
// of a key which is already known to hold secrets, so that the list of those stays in one place.
func TestRegisterMetaKeyValueRedactorUnknownKey(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		runtime.RegisterMetaKeyValueRedactor(runtime.MetaKeyTagToID(meta.Upgrade), func(value string) (string, bool) {
			return value, false
		})
	})
}
