// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile

import (
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	_ "github.com/siderolabs/talos/pkg/machinery/resources/network" // registers the redaction of the metal platform network config META key
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Redacted returns a copy of the profile with the secrets it carries taken out.
//
// It is meant for showing the profile (e.g. in a log), and should never be used to build an image.
//
// The META values are redacted by the same rules as the MetaKey resource, and the secrets in the
// embedded machine configuration are replaced the same way as the machine configuration is served
// to a reader. A machine configuration which can't be parsed is replaced in full, as there is no
// telling which parts of it are secrets.
func (p *Profile) Redacted() Profile {
	redacted := p.DeepCopy()

	redacted.Customization.MetaContents = RedactMetaValues(redacted.Customization.MetaContents)

	if redacted.Customization.EmbeddedMachineConfiguration != "" {
		redacted.Customization.EmbeddedMachineConfiguration = redactMachineConfiguration(redacted.Customization.EmbeddedMachineConfiguration)
	}

	return redacted
}

// RedactMetaValues returns a copy of META values with the secrets taken out.
func RedactMetaValues(values meta.Values) meta.Values {
	if values == nil {
		return nil
	}

	redacted := make(meta.Values, 0, len(values))

	for _, value := range values {
		value.Value, _ = runtime.RedactMetaValue(value.Key, value.Value)

		redacted = append(redacted, value)
	}

	return redacted
}

func redactMachineConfiguration(cfg string) string {
	provider, err := configloader.NewFromBytes([]byte(cfg))
	if err != nil {
		return constants.Redacted
	}

	out, err := provider.RedactSecrets(constants.Redacted).EncodeString(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return constants.Redacted
	}

	return out
}
