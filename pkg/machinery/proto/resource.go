// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package proto defines a functions to work with proto messages.
package proto

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/protoenc"
)

// ResourceSpecToProto converts a resource spec to a proto message.
func ResourceSpecToProto(i resource.Resource, o Message) error {
	marshaled, err := protoenc.Marshal(i.Spec())
	if err != nil {
		return err
	}

	return Unmarshal(marshaled, o)
}
